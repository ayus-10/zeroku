package api

type DeployRequest struct {
	Repo          string            `json:"repo"`
	GitRef        string            `json:"git_ref"`
	ContainerPort int               `json:"container_port"`
	Env           map[string]string `json:"env,omitempty"`
}
