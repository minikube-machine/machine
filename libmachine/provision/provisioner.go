package provision

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/docker/machine/libmachine/assets"
	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/cert"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/engine"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/docker/machine/libmachine/provision/pkgaction"
	"github.com/docker/machine/libmachine/provision/serviceaction"
	"github.com/docker/machine/libmachine/swarm"
	"github.com/docker/machine/libmachine/utils"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"k8s.io/minikube/pkg/minikube/command"
	"k8s.io/minikube/pkg/minikube/config"
)

var (
	provisioners          = make(map[string]*RegisteredProvisioner)
	detector     Detector = &StandardDetector{}
)

// for escaping systemd template specifiers (e.g. '%i'), which are not supported by minikube
var systemdSpecifierEscaper = strings.NewReplacer("%", "%%")

const (
	LastReleaseBeforeCEVersioning = "1.13.1"
)

type SSHCommander interface {
	// Short-hand for accessing an SSH command from the driver.
	SSHCommand(args string) (string, error)
}

type Detector interface {
	DetectProvisioner(d drivers.Driver) (Provisioner, error)
}

type StandardDetector struct{}

func SetDetector(newDetector Detector) {
	detector = newDetector
}

// Provisioner defines distribution specific actions
type Provisioner interface {
	fmt.Stringer
	SSHCommander

	// Create the files for the daemon to consume configuration settings (return struct of content and path)
	GenerateDockerOptions(dockerPort int) (*DockerOptions, error)

	// Get the directory where the settings files for docker are to be found
	GetDockerOptionsDir() string

	// Return the auth options used to configure remote connection for the daemon.
	GetAuthOptions() auth.Options

	// Get the swarm options associated with this host.
	GetSwarmOptions() swarm.Options

	// Run a package action e.g. install
	Package(name string, action pkgaction.PackageAction) error

	// Get Hostname
	Hostname() (string, error)

	// Set hostname
	SetHostname(hostname string) error

	// Figure out if this is the right provisioner to use based on /etc/os-release info
	CompatibleWithHost() bool

	// Do the actual provisioning piece:
	//     1. Set the hostname on the instance.
	//     2. Install Docker if it is not present.
	//     3. Configure the daemon to accept connections over TLS.
	//     4. Copy the needed certificates to the server and local config dir.
	//     5. Configure / activate swarm if applicable.
	Provision(swarmOptions swarm.Options, authOptions auth.Options, engineOptions engine.Options) error

	// Perform action on a named service e.g. stop
	Service(name string, action serviceaction.ServiceAction) error

	// Get the driver which is contained in the provisioner.
	GetDriver() drivers.Driver

	// Set the OS Release info depending on how it's represented
	// internally
	SetOsReleaseInfo(info *OsRelease)

	// Get the OS Release info for the current provisioner
	GetOsReleaseInfo() (*OsRelease, error)
}

// RegisteredProvisioner creates a new provisioner
type RegisteredProvisioner struct {
	New func(d drivers.Driver) Provisioner
}

func Register(name string, p *RegisteredProvisioner) {
	provisioners[name] = p
}

func DetectProvisioner(d drivers.Driver) (Provisioner, error) {
	return detector.DetectProvisioner(d)
}

func (detector StandardDetector) DetectProvisioner(d drivers.Driver) (Provisioner, error) {
	log.Info("Waiting for SSH to be available...")
	if err := drivers.WaitForSSH(d); err != nil {
		return nil, err
	}

	log.Info("Detecting the provisioner...")

	osReleaseOut, err := drivers.RunSSHCommandFromDriver(d, "cat /etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("Error getting SSH command: %s", err)
	}

	osReleaseInfo, err := NewOsRelease([]byte(osReleaseOut))
	if err != nil {
		return nil, fmt.Errorf("Error parsing /etc/os-release file: %s", err)
	}

	for _, p := range provisioners {
		provisioner := p.New(d)
		provisioner.SetOsReleaseInfo(osReleaseInfo)

		if provisioner.CompatibleWithHost() {
			log.Debugf("found compatible host: %s", osReleaseInfo.ID)
			return provisioner, nil
		}
	}

	return nil, ErrDetectionFailed
}

func rootFileSystemType(p SSHCommander) (string, error) {
	fs, err := p.SSHCommand("df --output=fstype / | tail -n 1")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(fs), nil
}

// updateUnit efficiently updates a systemd unit file
func updateUnit(p SSHCommander, name string, content string, dst string) error {
	klog.Infof("Updating %s unit: %s ...", name, dst)

	if _, err := p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s && printf %%s \"%s\" | sudo tee %s.new", path.Dir(dst), content, dst)); err != nil {
		return err
	}
	if _, err := p.SSHCommand(fmt.Sprintf("sudo diff -u %s %s.new || { sudo mv %s.new %s; sudo systemctl -f daemon-reload && sudo systemctl -f enable %s && sudo systemctl -f restart %s; }", dst, dst, dst, dst, name, name)); err != nil {
		return err
	}
	return nil
}

// escapeSystemdDirectives escapes special characters in the input variables used to create the
// systemd unit file, which would otherwise be interpreted as systemd directives. An example
// are template specifiers (e.g. '%i') which are predefined variables that get evaluated dynamically
// (see systemd man pages for more info). This is not supported by minikube, thus needs to be escaped.
func escapeSystemdDirectives(engineConfigContext *EngineConfigContext) {
	// escape '%' in Environment option so that it does not evaluate into a template specifier
	engineConfigContext.EngineOptions.Env = utils.ReplaceChars(engineConfigContext.EngineOptions.Env, systemdSpecifierEscaper)
	// input might contain whitespaces, wrap it in quotes
	engineConfigContext.EngineOptions.Env = utils.ConcatStrings(engineConfigContext.EngineOptions.Env, "\"", "\"")
}

func configureAuth(p Provisioner) error {
	klog.Infof("configureAuth start")
	start := time.Now()
	defer func() {
		klog.Infof("duration metric: configureAuth took %s", time.Since(start))
	}()

	driver := p.GetDriver()
	machineName := driver.GetMachineName()
	authOptions := p.GetAuthOptions()
	org := mcnutils.GetUsername() + "." + machineName
	bits := 2048

	ip, err := driver.GetIP()
	if err != nil {
		return errors.Wrap(err, "error getting ip during provisioning")
	}

	hostIP, err := driver.GetSSHHostname()
	if err != nil {
		return errors.Wrap(err, "error getting ssh hostname during provisioning")
	}

	if err := copyHostCerts(authOptions); err != nil {
		return err
	}

	hosts := authOptions.ServerCertSANs
	// The Host IP is always added to the certificate's SANs list
	hosts = append(hosts, ip, hostIP, "localhost", "127.0.0.1", "minikube", machineName)
	klog.Infof("generating server cert: %s ca-key=%s private-key=%s org=%s san=%s",
		authOptions.ServerCertPath,
		authOptions.CaCertPath,
		authOptions.CaPrivateKeyPath,
		org,
		hosts,
	)

	err = cert.GenerateCert(&cert.Options{
		Hosts:     hosts,
		CertFile:  authOptions.ServerCertPath,
		KeyFile:   authOptions.ServerKeyPath,
		CAFile:    authOptions.CaCertPath,
		CAKeyFile: authOptions.CaPrivateKeyPath,
		Org:       org,
		Bits:      bits,
	})

	if err != nil {
		return fmt.Errorf("error generating server cert: %v", err)
	}

	return copyRemoteCerts(authOptions, driver)
}

func setContainerRuntimeOptions(name string, p Provisioner) error {
	c, err := config.Load(name)
	if err != nil {
		return errors.Wrap(err, "getting cluster config")
	}

	switch c.KubernetesConfig.ContainerRuntime {
	case "crio", "cri-o":
		return setCrioOptions(p)
	case "containerd":
		return nil
	default:
		_, err := p.GenerateDockerOptions(engine.DefaultPort)
		return err
	}
}

func copyRemoteCerts(authOptions auth.Options, driver drivers.Driver) error {
	klog.Infof("copyRemoteCerts")

	remoteCerts := map[string]string{
		authOptions.CaCertPath:     authOptions.CaCertRemotePath,
		authOptions.ServerCertPath: authOptions.ServerCertRemotePath,
		authOptions.ServerKeyPath:  authOptions.ServerKeyRemotePath,
	}

	sshRunner := command.NewSSHRunner(driver)

	dirs := []string{}
	for _, dst := range remoteCerts {
		dirs = append(dirs, path.Dir(dst))
	}

	args := append([]string{"mkdir", "-p"}, dirs...)
	if _, err := sshRunner.RunCmd(exec.Command("sudo", args...)); err != nil {
		return err
	}

	for src, dst := range remoteCerts {
		f, err := assets.NewFileAsset(src, path.Dir(dst), filepath.Base(dst), "0640")
		if err != nil {
			return errors.Wrapf(err, "error copying %s to %s", src, dst)
		}
		defer func() {
			if err := f.Close(); err != nil {
				klog.Warningf("error closing the file %s: %v", f.GetSourcePath(), err)
			}
		}()

		if err := sshRunner.Copy(f); err != nil {
			return errors.Wrapf(err, "transferring file to machine %v", f)
		}
	}

	return nil
}

func copyHostCerts(authOptions auth.Options) error {
	klog.Infof("copyHostCerts")

	err := os.MkdirAll(authOptions.StorePath, 0700)
	if err != nil {
		klog.Errorf("mkdir failed: %v", err)
	}

	hostCerts := map[string]string{
		authOptions.CaCertPath:     path.Join(authOptions.StorePath, "ca.pem"),
		authOptions.ClientCertPath: path.Join(authOptions.StorePath, "cert.pem"),
		authOptions.ClientKeyPath:  path.Join(authOptions.StorePath, "key.pem"),
	}

	execRunner := command.NewExecRunner(false)
	for src, dst := range hostCerts {
		f, err := assets.NewFileAsset(src, path.Dir(dst), filepath.Base(dst), "0777")
		if err != nil {
			return errors.Wrapf(err, "open cert file: %s", src)
		}
		defer func() {
			if err := f.Close(); err != nil {
				klog.Warningf("error closing the file %s: %v", f.GetSourcePath(), err)
			}
		}()

		if err := execRunner.Copy(f); err != nil {
			return errors.Wrapf(err, "transferring file: %+v", f)
		}
	}

	return nil
}

func setCrioOptions(p SSHCommander) error {
	// pass through --insecure-registry
	var (
		crioOptsTmpl = `
CRIO_MINIKUBE_OPTIONS='{{ range .EngineOptions.InsecureRegistry }}--insecure-registry {{.}} {{ end }}'
`
		crioOptsPath = "/etc/sysconfig/crio.minikube"
	)
	t, err := template.New("crioOpts").Parse(crioOptsTmpl)
	if err != nil {
		return err
	}
	var crioOptsBuf bytes.Buffer
	if err := t.Execute(&crioOptsBuf, p); err != nil {
		return err
	}

	if _, err = p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s && printf %%s \"%s\" | sudo tee %s && sudo systemctl restart crio", path.Dir(crioOptsPath), crioOptsBuf.String(), crioOptsPath)); err != nil {
		return err
	}

	return nil
}
