package preprocess

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "more prompt glued to a device-alias line via CR",
			in:   "device-alias database\n\x1b[7m--More--\x1b[m\r  device-alias name X pwwn 10:00:00:00:c9:11:22:33\n",
			want: []string{"device-alias database", "  device-alias name X pwwn 10:00:00:00:c9:11:22:33"},
		},
		{
			name: "more prompt glued to a zone member line via CR",
			in:   "zone name Z vsan 10\n\x1b[7m--More--\x1b[m\r    member fcalias Y\n    member pwwn 50:06:0e:80:04:7c:00:01\n",
			want: []string{"zone name Z vsan 10", "    member fcalias Y", "    member pwwn 50:06:0e:80:04:7c:00:01"},
		},
		{
			name: "CRLF line endings normalized",
			in:   "line one\r\nline two\r\n",
			want: []string{"line one", "line two"},
		},
		{
			name: "assorted ANSI sequences stripped, text intact",
			in:   "\x1b[Khello \x1b[1;32mworld\x1b[0m done\n",
			want: []string{"hello world done"},
		},
		{
			name: "standalone pager prompts dropped",
			in:   "a\n--More--\nb\n --More-- \nc\n------ More ------\nd\n",
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "a line merely containing the word more is kept",
			in:   "  device-alias name moreStorage pwwn 10:00:00:00:c9:11:22:33\n",
			want: []string{"  device-alias name moreStorage pwwn 10:00:00:00:c9:11:22:33"},
		},
		{
			name: "empty input yields empty slice",
			in:   "",
			want: []string{},
		},
		{
			name: "input that is only pager prompts yields empty slice",
			in:   "--More--\n--More--\n",
			want: []string{},
		},
		{
			name: "artifact-free config returned unchanged",
			in:   "device-alias database\n  device-alias name X pwwn 10:00:00:00:c9:11:22:33\ndevice-alias commit\n",
			want: []string{"device-alias database", "  device-alias name X pwwn 10:00:00:00:c9:11:22:33", "device-alias commit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Clean(strings.NewReader(tt.in))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
