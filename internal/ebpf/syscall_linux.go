//go:build linux

package ebpf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	bpfMapCreate      = 0
	bpfMapLookupElem  = 1
	bpfProgLoad       = 5
	bpfMapTypeArray   = 2
	bpfProgTracepoint = 5

	perfTypeTracepoint = 2
	perfFlagFDCloexec  = 1 << 3
	perfIOCEnable      = 0x2400
	perfIOCSetBPF      = 0x40042408
)

type bpfInsn struct {
	Code   uint8
	DstSrc uint8
	Off    int16
	Imm    int32
}

func reg(dst, src uint8) uint8 { return (src << 4) | dst }

func counterProgram(mapFD int) []bpfInsn {
	// r1 = map pointer (pseudo map FD)
	// *(u32 *)(r10-4) = 0
	// r2 = &key
	// r0 = map_lookup_elem(r1, r2)
	// if r0 != NULL: atomic_add(*(u64*)r0, 1)
	return []bpfInsn{
		{Code: 0x18, DstSrc: reg(1, 1), Imm: int32(mapFD)}, // LD_DW_IMM r1, map fd
		{},
		{Code: 0x62, DstSrc: reg(10, 0), Off: -4, Imm: 0}, // ST_W [r10-4], 0
		{Code: 0xbf, DstSrc: reg(2, 10)},                  // MOV64 r2, r10
		{Code: 0x07, DstSrc: reg(2, 0), Imm: -4},          // ADD64 r2, -4
		{Code: 0x85, Imm: 1},                              // CALL map_lookup_elem
		{Code: 0x15, DstSrc: reg(0, 0), Off: 2, Imm: 0},   // JEQ r0, 0, +2
		{Code: 0xb7, DstSrc: reg(1, 0), Imm: 1},           // MOV64 r1, 1
		{Code: 0xdb, DstSrc: reg(0, 1)},                   // XADD_DW [r0], r1
		{Code: 0xb7, DstSrc: reg(0, 0), Imm: 0},           // MOV64 r0, 0
		{Code: 0x95},                                      // EXIT
	}
}

func attachCounterProbe(root string, cpus []int, definition Probe) (*kernelProbe, error) {
	id, err := tracepointID(root, definition)
	if err != nil {
		return nil, err
	}
	mapFD, err := createCounterMap("tf_" + shortSignal(definition.Signal))
	if err != nil {
		return nil, fmt.Errorf("create BPF counter map: %w", err)
	}
	progFD, err := loadCounterProgram(mapFD, "tf_"+shortSignal(definition.Signal))
	if err != nil {
		_ = closeFD(mapFD)
		return nil, fmt.Errorf("load BPF tracepoint program: %w", err)
	}
	probe := &kernelProbe{definition: definition, mapFD: mapFD, progFD: progFD}
	for _, cpu := range cpus {
		perfFD, openErr := openTracepoint(id, cpu)
		if openErr != nil {
			probeClose(probe)
			return nil, fmt.Errorf("open tracepoint on cpu %d: %w", cpu, openErr)
		}
		if err := ioctlFD(perfFD, perfIOCSetBPF, uintptr(progFD)); err != nil {
			_ = closeFD(perfFD)
			probeClose(probe)
			return nil, fmt.Errorf("attach BPF program on cpu %d: %w", cpu, err)
		}
		if err := ioctlFD(perfFD, perfIOCEnable, 0); err != nil {
			_ = closeFD(perfFD)
			probeClose(probe)
			return nil, fmt.Errorf("enable tracepoint on cpu %d: %w", cpu, err)
		}
		probe.perfFDs = append(probe.perfFDs, perfFD)
	}
	return probe, nil
}

func probeClose(probe *kernelProbe) {
	for _, fd := range probe.perfFDs {
		_ = closeFD(fd)
	}
	if probe.progFD >= 0 {
		_ = closeFD(probe.progFD)
	}
	if probe.mapFD >= 0 {
		_ = closeFD(probe.mapFD)
	}
}

func shortSignal(signal string) string {
	switch signal {
	case SignalProcessExec:
		return "exec"
	case SignalSocketConnect:
		return "connect"
	case SignalTCPRetransmit:
		return "retrans"
	default:
		return "counter"
	}
}

func createCounterMap(name string) (int, error) {
	attr := make([]byte, 72)
	binary.LittleEndian.PutUint32(attr[0:4], bpfMapTypeArray)
	binary.LittleEndian.PutUint32(attr[4:8], 4)
	binary.LittleEndian.PutUint32(attr[8:12], 8)
	binary.LittleEndian.PutUint32(attr[12:16], 1)
	copy(attr[28:44], []byte(name))
	fd, err := bpfSyscall(bpfMapCreate, attr)
	return int(fd), err
}

func loadCounterProgram(mapFD int, name string) (int, error) {
	instructions := counterProgram(mapFD)
	license := append([]byte("GPL"), 0)
	logBuf := make([]byte, 64*1024)
	attr := make([]byte, 72)
	binary.LittleEndian.PutUint32(attr[0:4], bpfProgTracepoint)
	binary.LittleEndian.PutUint32(attr[4:8], uint32(len(instructions)))
	binary.LittleEndian.PutUint64(attr[8:16], uint64(uintptr(unsafe.Pointer(&instructions[0]))))
	binary.LittleEndian.PutUint64(attr[16:24], uint64(uintptr(unsafe.Pointer(&license[0]))))
	binary.LittleEndian.PutUint32(attr[24:28], 1)
	binary.LittleEndian.PutUint32(attr[28:32], uint32(len(logBuf)))
	binary.LittleEndian.PutUint64(attr[32:40], uint64(uintptr(unsafe.Pointer(&logBuf[0]))))
	copy(attr[48:64], []byte(name))
	fd, err := bpfSyscall(bpfProgLoad, attr)
	runtime.KeepAlive(instructions)
	runtime.KeepAlive(license)
	runtime.KeepAlive(logBuf)
	if err != nil {
		text := string(logBuf)
		if nul := indexByte(logBuf, 0); nul >= 0 {
			text = string(logBuf[:nul])
		}
		if text != "" {
			return -1, fmt.Errorf("%w (verifier: %s)", err, text)
		}
		return -1, err
	}
	return int(fd), nil
}

func lookupCounter(mapFD int) (uint64, error) {
	key := uint32(0)
	value := uint64(0)
	attr := make([]byte, 32)
	binary.LittleEndian.PutUint32(attr[0:4], uint32(mapFD))
	binary.LittleEndian.PutUint64(attr[8:16], uint64(uintptr(unsafe.Pointer(&key))))
	binary.LittleEndian.PutUint64(attr[16:24], uint64(uintptr(unsafe.Pointer(&value))))
	_, err := bpfSyscall(bpfMapLookupElem, attr)
	runtime.KeepAlive(&key)
	runtime.KeepAlive(&value)
	return value, err
}

func bpfSyscall(command uintptr, attr []byte) (uintptr, error) {
	if sysBPF == 0 {
		return 0, errors.New("BPF syscall is unsupported on this Linux architecture")
	}
	result, _, errno := syscall.Syscall(sysBPF, command, uintptr(unsafe.Pointer(&attr[0])), uintptr(len(attr)))
	runtime.KeepAlive(attr)
	if errno != 0 {
		return 0, errno
	}
	return result, nil
}

func openTracepoint(id uint64, cpu int) (int, error) {
	attr := make([]byte, 128)
	binary.LittleEndian.PutUint32(attr[0:4], perfTypeTracepoint)
	binary.LittleEndian.PutUint32(attr[4:8], uint32(len(attr)))
	binary.LittleEndian.PutUint64(attr[8:16], id)
	// disabled=1; enable explicitly after attaching the BPF program.
	binary.LittleEndian.PutUint64(attr[40:48], 1)
	fd, _, errno := syscall.Syscall6(syscall.SYS_PERF_EVENT_OPEN,
		uintptr(unsafe.Pointer(&attr[0])), ^uintptr(0), uintptr(cpu), ^uintptr(0), perfFlagFDCloexec, 0)
	runtime.KeepAlive(attr)
	if errno != 0 {
		return -1, errno
	}
	return int(fd), nil
}

func ioctlFD(fd int, request, value uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, value)
	if errno != 0 {
		return errno
	}
	return nil
}

func closeFD(fd int) error {
	if fd < 0 {
		return nil
	}
	return syscall.Close(fd)
}

func indexByte(value []byte, target byte) int {
	for index, current := range value {
		if current == target {
			return index
		}
	}
	return -1
}

// ValidateKernelPrograms validates the dependency-free eBPF instruction
// templates without requiring BPF privileges. Live kernel verification still
// requires Start(), which passes the program through the kernel verifier.
func ValidateKernelPrograms() error {
	instructions := counterProgram(7)
	if len(instructions) != 11 {
		return fmt.Errorf("unexpected counter program length: %d", len(instructions))
	}
	if instructions[0].Code != 0x18 || instructions[1] != (bpfInsn{}) || instructions[len(instructions)-1].Code != 0x95 {
		return errors.New("counter program does not contain the expected map-load/exit envelope")
	}
	if instructions[8].Code != 0xdb {
		return errors.New("counter program is missing atomic 64-bit increment")
	}
	return nil
}
