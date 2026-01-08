module github.com/docker/machine

go 1.25

require (
	github.com/aregm/cpuid v0.0.0-20181003105527-1a4a6f06a1c6
	github.com/hectane/go-acl v0.0.0-20190604041725-da78bae5fc95
	github.com/moby/term v0.5.0
	github.com/sayboras/dockerclient v1.0.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.46.0
	golang.org/x/sys v0.39.0
	golang.org/x/term v0.38.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker v0.0.0-20180621001606-093424bec097 // indirect
	github.com/docker/go-units v0.0.0-20151230175859-0bbddae09c5a // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
)

replace (
	github.com/Nvveen/Gotty => github.com/ijc25/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20180718150940-a3ef7e9a9bda+incompatible
	github.com/docker/go-units => github.com/docker/go-units v0.3.2-0.20170127094116-9e638d38cf69
)
