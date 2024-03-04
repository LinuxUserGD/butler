package butlerd

import "github.com/LinuxUserGD/butler/manager"

func (rc *RequestContext) HostEnumerator() manager.HostEnumerator {
	return manager.DefaultHostEnumerator()
}
