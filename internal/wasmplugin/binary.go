package wasmplugin

import "fmt"

// VerifyEntrypoint performs a minimal WebAssembly section parse and requires an
// exported function with the manifest entrypoint name. It does not execute code.
func VerifyEntrypoint(module []byte, entrypoint string) error {
	if len(module) < 8 {
		return fmt.Errorf("truncated WebAssembly module")
	}
	pos := 8
	for pos < len(module) {
		sectionID := module[pos]
		pos++
		size, n, err := readULEB(module[pos:])
		if err != nil {
			return err
		}
		pos += n
		end := pos + int(size)
		if end > len(module) {
			return fmt.Errorf("truncated WebAssembly section")
		}
		if sectionID == 7 {
			payload := module[pos:end]
			count, used, err := readULEB(payload)
			if err != nil {
				return err
			}
			payload = payload[used:]
			for i := uint32(0); i < count; i++ {
				nameLen, used, err := readULEB(payload)
				if err != nil {
					return err
				}
				payload = payload[used:]
				if int(nameLen)+2 > len(payload) {
					return fmt.Errorf("truncated WebAssembly export")
				}
				name := string(payload[:nameLen])
				payload = payload[nameLen:]
				kind := payload[0]
				payload = payload[1:]
				_, used, err = readULEB(payload)
				if err != nil {
					return err
				}
				payload = payload[used:]
				if kind == 0 && name == entrypoint {
					return nil
				}
			}
		}
		pos = end
	}
	return fmt.Errorf("WebAssembly module does not export function %q", entrypoint)
}

func readULEB(data []byte) (uint32, int, error) {
	var value uint32
	for i := 0; i < len(data) && i < 5; i++ {
		b := data[i]
		value |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("invalid WebAssembly unsigned LEB128 value")
}
