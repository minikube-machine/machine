package provision

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/cert"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

type DockerOptions struct {
	EngineOptions     string
	EngineOptionsPath string
}

func installDockerGeneric(p Provisioner, baseURL string) error {
	// install docker - until cloudinit we use ubuntu everywhere so we
	// just install it using the docker repos
	if output, err := p.SSHCommand(fmt.Sprintf("if ! type docker; then curl -sSL %s | sh -; fi", baseURL)); err != nil {
		return fmt.Errorf("error installing docker: %s", output)
	}

	return nil
}

func makeDockerOptionsDir(p Provisioner) error {
	dockerDir := p.GetDockerOptionsDir()
	if _, err := p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s", dockerDir)); err != nil {
		return err
	}

	return nil
}

func setRemoteAuthOptions(p Provisioner) auth.Options {
	dockerDir := p.GetDockerOptionsDir()
	authOptions := p.GetAuthOptions()

	// due to windows clients, we cannot use filepath.Join as the paths
	// will be mucked on the linux hosts
	authOptions.CaCertRemotePath = path.Join(dockerDir, "ca.pem")
	authOptions.ServerCertRemotePath = path.Join(dockerDir, "server.pem")
	authOptions.ServerKeyRemotePath = path.Join(dockerDir, "server-key.pem")

	return authOptions
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

	for src, dst := range hostCerts {
		if err := mcnutils.CopyFile(src, dst); err != nil {
			return errors.Wrapf(err, "transferring file: %+v", dst)
		}
	}

	return nil
}

func copyRemoteCerts(authOptions auth.Options, driver drivers.Driver, p Provisioner) error {
	klog.Infof("copyRemoteCerts")

	remoteCerts := map[string]string{
		authOptions.CaCertPath:     authOptions.CaCertRemotePath,
		authOptions.ServerCertPath: authOptions.ServerCertRemotePath,
		authOptions.ServerKeyPath:  authOptions.ServerKeyRemotePath,
	}

	dirs := []string{}
	for _, dst := range remoteCerts {
		dirs = append(dirs, path.Dir(dst))
	}

	args := append([]string{"mkdir", "-p"}, dirs...)
	if _, err := p.SSHCommand("sudo " + strings.Join(args, " ")); err != nil {
		return err
	}

	certTransferCmdFmt := "printf '%%s' '%s' | sudo tee %s"
	for src, dst := range remoteCerts {
		fileContent, err := ioutil.ReadFile(src)
		if err != nil {
			return errors.Wrapf(err, "while reading %+v's content", src)
		}

		if _, err := p.SSHCommand(fmt.Sprintf(certTransferCmdFmt, fileContent, dst)); err != nil {
			return errors.Wrapf(err, "transferring %+v to machine", src)
		}
	}

	return nil
}

func ConfigureAuth(p Provisioner) error {
	klog.Infof("configureAuth start")
	start := time.Now()
	defer func() {
		klog.Infof("duration metric: configureAuth took %s", time.Since(start))
	}()

	var err error

	driver := p.GetDriver()
	machineName := driver.GetMachineName()
	authOptions := p.GetAuthOptions()
	org := mcnutils.GetUsername() + "." + machineName
	bits := 2048

	ip, err := driver.GetIP()
	if err != nil {
		return errors.Wrap(err, "error while getting ip during provisioning")
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

	// TODO: Switch to passing just authOptions to this func
	// instead of all these individual fields
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
		return fmt.Errorf("error generating server cert: %s", err)
	}

	return copyRemoteCerts(authOptions, driver, p)
}

func matchNetstatOut(reDaemonListening, netstatOut string) bool {
	// TODO: I would really prefer this be a Scanner directly on
	// the STDOUT of the executed command than to do all the string
	// manipulation hokey-pokey.
	//
	// TODO: Unit test this matching.
	for _, line := range strings.Split(netstatOut, "\n") {
		match, err := regexp.MatchString(reDaemonListening, line)
		if err != nil {
			log.Warnf("Regex warning: %s", err)
		}
		if match && line != "" {
			return true
		}
	}

	return false
}

func decideStorageDriver(p Provisioner, defaultDriver, suppliedDriver string) (string, error) {
	if suppliedDriver != "" {
		return suppliedDriver, nil
	}
	bestSuitedDriver := ""

	defer func() {
		if bestSuitedDriver != "" {
			log.Debugf("No storagedriver specified, using %s\n", bestSuitedDriver)
		}
	}()

	if defaultDriver != "aufs" {
		bestSuitedDriver = defaultDriver
	} else {
		remoteFilesystemType, err := getFilesystemType(p, "/var/lib")
		if err != nil {
			return "", err
		}
		if remoteFilesystemType == "btrfs" {
			bestSuitedDriver = "btrfs"
		} else {
			bestSuitedDriver = defaultDriver
		}
	}
	return bestSuitedDriver, nil

}

func getFilesystemType(p Provisioner, directory string) (string, error) {
	statCommandOutput, err := p.SSHCommand("stat -f -c %T " + directory)
	if err != nil {
		err = fmt.Errorf("Error looking up filesystem type: %s", err)
		return "", err
	}

	fstype := strings.TrimSpace(statCommandOutput)
	return fstype, nil
}

func checkDaemonUp(p Provisioner, dockerPort int) func() bool {
	reDaemonListening := fmt.Sprintf(":%d\\s+.*:.*", dockerPort)
	return func() bool {
		// HACK: Check netstat's output to see if anyone's listening on the Docker API port.
		netstatOut, err := p.SSHCommand("if ! type netstat 1>/dev/null; then ss -tln; else netstat -tln; fi")
		if err != nil {
			log.Warnf("Error running SSH command: %s", err)
			return false
		}

		return matchNetstatOut(reDaemonListening, netstatOut)
	}
}

func WaitForDocker(p Provisioner, dockerPort int) error {
	if err := mcnutils.WaitForSpecific(checkDaemonUp(p, dockerPort), 10, 3*time.Second); err != nil {
		return NewErrDaemonAvailable(err)
	}

	return nil
}

// DockerClientVersion returns the version of the Docker client on the host
// that ssh is connected to, e.g. "1.12.1".
func DockerClientVersion(ssh SSHCommander) (string, error) {
	// `docker version --format {{.Client.Version}}` would be preferable, but
	// that fails if the server isn't running yet.
	//
	// output is expected to be something like
	//
	//     Docker version 1.12.1, build 7a86f89
	output, err := ssh.SSHCommand("docker --version")
	if err != nil {
		return "", err
	}

	words := strings.Fields(output)
	if len(words) < 3 || words[0] != "Docker" || words[1] != "version" {
		return "", fmt.Errorf("DockerClientVersion: cannot parse version string from %q", output)
	}

	return strings.TrimRight(words[2], ","), nil
}

func waitForLockAptGetUpdate(ssh SSHCommander) error {
	return waitForLock(ssh, "sudo apt-get update")
}

func waitForLock(ssh SSHCommander, cmd string) error {
	var sshErr error
	err := mcnutils.WaitFor(func() bool {
		_, sshErr = ssh.SSHCommand(cmd)
		if sshErr != nil {
			if strings.Contains(sshErr.Error(), "Could not get lock") {
				sshErr = nil
				return false
			}
			return true
		}
		return true
	})
	if sshErr != nil {
		return fmt.Errorf("Error running %q: %s", cmd, sshErr)
	}
	if err != nil {
		return fmt.Errorf("Failed to obtain lock: %s", err)
	}
	return nil
}
