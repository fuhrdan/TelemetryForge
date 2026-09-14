//go:build linux

package ebpf

import "testing"

func TestCounterProgramTemplate(t *testing.T) {
	if err := ValidateKernelPrograms(); err != nil {
		t.Fatal(err)
	}
	program := counterProgram(42)
	if program[0].Imm != 42 {
		t.Fatalf("map fd imm=%d", program[0].Imm)
	}
	if program[6].Off != 2 {
		t.Fatalf("nil-map branch offset=%d", program[6].Off)
	}
}

func TestParseSignalsDeduplicatesSupportedSignals(t *testing.T) {
	got, err := ParseSignalsStrict("process_exec,socket_connect,process_exec")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != SignalProcessExec || got[1] != SignalSocketConnect {
		t.Fatalf("signals=%v", got)
	}
}

func TestParseSignalsRejectsUnknownSignal(t *testing.T) {
	if _, err := ParseSignalsStrict("process_exec,unknown"); err == nil {
		t.Fatal("expected unsupported signal error")
	}
}
