//go:build !race && (amd64 || arm64 || loong64 || mips64 || mips64le || ppc64 || ppc64le || riscv64 || s390x)

package json

import (
	"github.com/bytedance/sonic"
)

var (
	Marshal   = sonic.Marshal
	Unmarshal = sonic.Unmarshal
)

type RawMessage = sonic.NoCopyRawMessage
