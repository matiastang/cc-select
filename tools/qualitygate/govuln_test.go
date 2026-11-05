package main

import (
	"testing"
)

func TestParseGovulnSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		output  string
		want    int
		wantErr bool
	}{
		{
			name:   "无漏洞",
			output: "No vulnerabilities found.\n",
			want:   0,
		},
		{
			name:   "标准库可触达漏洞",
			output: "Your code is affected by 32 vulnerabilities from the Go standard library.\nThis scan also found 6 vulnerabilities in packages you import and 9\nvulnerabilities in modules you require, but your code doesn't appear to call\nthese vulnerabilities.\n",
			want:   32,
		},
		{
			name:   "依赖可触达漏洞",
			output: "Your code is affected by 2 vulnerabilities from 1 module.\n",
			want:   2,
		},
		{
			name:    "无法识别的摘要（视为配置错误，不假绿）",
			output:  "some unexpected output\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseGovulnSummary(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGovulnSummary: %v", err)
			}
			if got != tt.want {
				t.Fatalf("affected=%d, want %d", got, tt.want)
			}
		})
	}
}
