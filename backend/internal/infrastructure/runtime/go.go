package runtime

import goruntime "runtime"

type GoRuntime struct{}

func (GoRuntime) Version() string {
	return goruntime.Version()
}
