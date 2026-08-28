package query

import "testing"

func TestParamsOmitUnsetValues(t *testing.T) {
	p := New()
	p.Set("empty", "")
	p.Set("full", "x")
	p.SetList("none", nil)
	p.SetList("list", []string{"a", "b"})
	p.SetInt("zero", 0)
	p.SetInt("n", 7)
	p.SetBool("nilbool", nil)
	f := false
	p.SetBool("f", &f)

	v := p.Values()
	for _, absent := range []string{"empty", "none", "zero", "nilbool"} {
		if _, ok := v[absent]; ok {
			t.Errorf("%s should be omitted", absent)
		}
	}
	if v.Get("full") != "x" || v.Get("list") != "a,b" || v.Get("n") != "7" || v.Get("f") != "false" {
		t.Errorf("values = %v", v)
	}
}
