package parser

import "testing"

func TestParseList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]int // name → expected qty
	}{
		{
			name:  "plain names",
			input: "Sol Ring\nLightning Bolt\n",
			want:  map[string]int{"Sol Ring": 1, "Lightning Bolt": 1},
		},
		{
			name:  "quantity prefix",
			input: "1 Sol Ring\n3 Forest\n",
			want:  map[string]int{"Sol Ring": 1, "Forest": 3},
		},
		{
			name:  "quantity with x",
			input: "2x Counterspell",
			want:  map[string]int{"Counterspell": 2},
		},
		{
			name:  "quantity with X uppercase",
			input: "2X Lightning Bolt",
			want:  map[string]int{"Lightning Bolt": 2},
		},
		{
			name:  "ignores double-slash comments",
			input: "// deck list\nSol Ring\n",
			want:  map[string]int{"Sol Ring": 1},
		},
		{
			name:  "ignores hash comments",
			input: "# header\nSol Ring\n",
			want:  map[string]int{"Sol Ring": 1},
		},
		{
			name:  "ignores blank lines",
			input: "\n\nSol Ring\n\n",
			want:  map[string]int{"Sol Ring": 1},
		},
		{
			name:  "accumulates duplicates",
			input: "1 Sol Ring\n2 Sol Ring",
			want:  map[string]int{"Sol Ring": 3},
		},
		{
			name:  "empty input",
			input: "",
			want:  map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseList(tt.input)
			gotMap := make(map[string]int, len(got))
			for _, e := range got {
				gotMap[e.Name] = e.Qty
			}
			if len(gotMap) != len(tt.want) {
				t.Errorf("got %d entries, want %d: %v vs %v", len(gotMap), len(tt.want), gotMap, tt.want)
				return
			}
			for name, qty := range tt.want {
				if gotMap[name] != qty {
					t.Errorf("card %q: got qty %d, want %d", name, gotMap[name], qty)
				}
			}
		})
	}
}
