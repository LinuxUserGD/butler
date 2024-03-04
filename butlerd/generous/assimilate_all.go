package main

func (s *scope) assimilateAll() {
	must(s.assimilate("github.com/LinuxUserGD/butler/butlerd", "types_launch.go"))
	must(s.assimilate("github.com/LinuxUserGD/butler/butlerd", "types.go"))
	must(s.assimilate("github.com/LinuxUserGD/butler/manager", "types_host.go"))

	must(s.assimilate("github.com/itchio/dash", "types.go"))

	must(s.assimilate("github.com/itchio/go-itchio", "types.go"))

	must(s.assimilate("github.com/LinuxUserGD/hush", "event_types.go"))
	must(s.assimilate("github.com/LinuxUserGD/hush/bfs", "receipt.go"))
	must(s.assimilate("github.com/LinuxUserGD/hush/manifest", "manifest.go"))

	must(s.assimilate("github.com/itchio/ox", "runtime.go"))
}
