package install

type CmdOptions struct {
	Codefresh struct {
		Host string
		Auth struct {
			Token string
		}
	}

	Kube struct {
		ManifestPath string
		Namespace    string
		Context      string
		ConfigPath   string
	}
	Argo struct {
		Password string
	}
}
