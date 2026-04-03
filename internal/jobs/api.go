package jobs

// API is the Wails-facing surface for job inspection and control.
type API struct {
	manager *Manager
}

func NewAPI(manager *Manager) *API {
	return &API{manager: manager}
}

func (api *API) JobCancel(id string) {
	api.manager.Cancel(id)
}

func (api *API) JobDismiss(id string) {
	api.manager.Dismiss(id)
}

func (api *API) JobGet(id string) *Job {
	job, _ := api.manager.Get(id)
	return job
}

func (api *API) JobList() []*Job {
	return api.manager.List()
}
