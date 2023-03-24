package naming

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"
)

func MustGenerateK8sName(prefix string) string {
	if prefix != "" {
		rfcCheckErrs := validation.IsDNS1123Label(prefix)
		if rfcCheckErrs != nil {
			panic(fmt.Sprintf("prefix is not an RFC 1123 compatible identifier: %v", rfcCheckErrs))
		}
	}

	now := time.Now().String()
	h := sha256.New()
	h.Write([]byte(now))
	hash := hex.EncodeToString(h.Sum(nil))

	delimiter := ""
	if prefix != "" {
		delimiter = "-"
	}

	return prefix + delimiter + hash[0:8]
}
