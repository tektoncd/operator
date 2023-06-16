package repository

func New(host, owner, name string) repo {
	return repo{host: host, owner: owner, name: name}
}

// Implements repository.Repository interface.
type repo struct {
	host  string
	owner string
	name  string
}

func (r repo) Host() string {
	return r.host
}

func (r repo) Owner() string {
	return r.owner
}

func (r repo) Name() string {
	return r.name
}
