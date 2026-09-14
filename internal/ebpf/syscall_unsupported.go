//go:build !linux

package ebpf

import "errors"

func ValidateKernelPrograms() error { return errors.New("eBPF kernel programs require Linux") }
func attachCounterProbe(string, []int, Probe) (*kernelProbe, error) {
	return nil, errors.New("eBPF requires Linux")
}
func lookupCounter(int) (uint64, error) { return 0, errors.New("eBPF requires Linux") }
func closeFD(int) error                 { return nil }
