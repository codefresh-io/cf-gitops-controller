package install

type CmdOptions struct {
	Git struct {
		Auth struct {
			Type string
			Pass string
		}
		Integration string
		RepoUrl     string
	}

	Host struct {
		HttpProxy  string
		HttpsProxy string
	}

	Codefresh struct {
		Host   string
		Suffix string
		Auth   struct {
			Token string
		}
		Clusters []string
	}

	Kube struct {
		ManifestPath string
		Namespace    string
		Context      string
		ConfigPath   string
		InCluster    bool
	}
	Argo struct {
		Token    string
		Host     string
		Password string
		Username string
	}

	Controller struct {
		LoadBalancer bool
	}
}
