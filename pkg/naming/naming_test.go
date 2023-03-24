package naming

import (
	"regexp"
	"testing"
)

func TestMustGenerateK8sName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{"- delimits prefix from hash", "qwer", "qwer-[a-f0-9]{8}"},
		{"empty prefix lead to hash", "", "[a-f0-9]{8}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := regexp.MustCompile(tt.want)
			if got := MustGenerateK8sName(tt.prefix); !r.MatchString(got) {
				t.Errorf("MustGenerateK8sName() = %v, must match regexp: %v", got, tt.want)
			}
		})
	}

	t.Run("should panic on invalid name", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic")
			}
		}()

		_ = MustGenerateK8sName("ÜŞ$")
	})
}
