package skills

import "context"

type Descriptor struct {
	Name              string
	Description       string
	Trigger           string
	RequiredTools     []string
	SystemPromptPatch string
}

type Registry interface {
	List(ctx context.Context) ([]Descriptor, error)
}

type StaticRegistry struct {
	descriptors []Descriptor
}

func NewStaticRegistry(descriptors []Descriptor) *StaticRegistry {
	return &StaticRegistry{descriptors: descriptors}
}

func (r *StaticRegistry) List(context.Context) ([]Descriptor, error) {
	return r.descriptors, nil
}
