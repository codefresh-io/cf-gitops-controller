package install

type CmdOptions struct {
	ManifestPath string

	Kube struct {
		Namespace  string
		Context    string
		ConfigPath string
	}
	Argo struct {
		Password string
	}
}
