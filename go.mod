module github.com/docker/machine

go 1.13

require (
	github.com/Azure/azure-sdk-for-go v5.0.0-beta+incompatible
	github.com/Azure/go-autorest v7.2.1+incompatible
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/aws/aws-sdk-go v1.4.10
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/bugsnag/bugsnag-go v0.0.0-20151120182711-02e952891c52
	github.com/bugsnag/osext v0.0.0-20130617224835-0dd3f918b21b // indirect
	github.com/bugsnag/panicwrap v0.0.0-20160118154447-aceac81c6e2f // indirect
	github.com/cenkalti/backoff v0.0.0-20141124221459-9831e1e25c87 // indirect
	github.com/codegangsta/cli v0.0.0-20151120215642-0302d3914d2a
	github.com/dgrijalva/jwt-go v0.0.0-20160831183534-24c63f56522a // indirect
	github.com/digitalocean/godo v0.0.0-20170317202744-d59ed2fe842b
	github.com/docker/docker v0.0.0-20180621001606-093424bec097 // indirect
	github.com/docker/go-units v0.0.0-20151230175859-0bbddae09c5a // indirect
	github.com/exoscale/egoscale v0.9.23
	github.com/go-ini/ini v0.0.0-20151124192405-03e0e7d51a13 // indirect
	github.com/golang/protobuf v0.0.0-20160221214941-3c84672111d9 // indirect
	github.com/google/go-querystring v0.0.0-20140804062624-30f7a39f4a21 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hectane/go-acl v0.0.0-20190604041725-da78bae5fc95
	github.com/intel-go/cpuid v0.0.0-20181003105527-1a4a6f06a1c6
	github.com/jinzhu/copier v0.0.0-20180308034124-7e38e58719c3 // indirect
	github.com/jmespath/go-jmespath v0.0.0-20151117175822-3433f3ea46d9 // indirect
	github.com/juju/loggo v1.0.0 // indirect
	github.com/mitchellh/mapstructure v0.0.0-20140721150620-740c764bc614 // indirect
	github.com/moby/term v0.0.0-20200416134343-063f2cd0b49d
	github.com/rackspace/gophercloud v0.0.0-20150408191457-ce0f487f6747
	github.com/samalba/dockerclient v0.0.0-20151231000007-f661dd4754aa
	github.com/skarademir/naturalsort v0.0.0-20150715044055-69a5d87bef62
	github.com/smartystreets/goconvey v1.7.2 // indirect
	github.com/stretchr/testify v1.2.2
	github.com/tent/http-link-go v0.0.0-20130702225549-ac974c61c2f9 // indirect
	github.com/vmware/govcloudair v0.0.2
	github.com/vmware/govmomi v0.6.2
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/net v0.0.0-20190311183353-d8887717615a
	golang.org/x/oauth2 v0.0.0-20151117210313-442624c9ec92
	golang.org/x/sys v0.0.0-20200302150141-5c8b2ff67527
	google.golang.org/api v0.0.0-20180213000552-87a2f5c77b36
	google.golang.org/appengine v0.0.0-20160205025855-6a436539be38 // indirect
	google.golang.org/cloud v0.0.0-20151119220103-975617b05ea8 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
)

replace github.com/samalba/dockerclient => github.com/sayboras/dockerclient v1.0.0
