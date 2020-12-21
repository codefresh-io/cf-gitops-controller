package install

type CmdOptions struct {
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
