package main

import "testing"

func TestCoreUpdateRequestValidate(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{in: "latest", want: "latest", ok: true},
		{in: "v25.8.31", want: "v25.8.31", ok: true},
		{in: "25.8.31", want: "v25.8.31", ok: true},
		{in: "", ok: false},
		{in: "v25.8", ok: false},
		{in: "newest", ok: false},
		{in: "latest --pre-release", ok: false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			req := coreUpdateRequest{CoreVersion: c.in}
			err := req.validate()
			if !c.ok {
				if err == nil {
					t.Fatalf("expected %q to be rejected", c.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected %q to be accepted, got %v", c.in, err)
			}
			if req.CoreVersion != c.want {
				t.Fatalf("expected %q to become %q, got %q", c.in, c.want, req.CoreVersion)
			}
		})
	}
}
