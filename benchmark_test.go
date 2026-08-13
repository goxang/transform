package transform_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/goxang/transform"
)

// ========== Benchmark types ==========

type benchFlat struct {
	Name   string `json:"name"`
	Email  string `json:"email" transform:"upper"`
	Secret string `json:"secret" transform:"mask"`
	Phone  string `json:"phone"`
	Amount int    `json:"amount"`
}

type benchNested struct {
	Inner benchFlat `json:"inner"`
	ID    string    `json:"id"`
}

type benchDeepL3 struct {
	Secret string `json:"secret" transform:"upper"`
}
type benchDeepL2 struct{ L3 benchDeepL3 }
type benchDeepL1 struct{ L1 benchDeepL2 }

type benchSlice struct {
	Items []benchFlat `json:"items"`
}

type benchMap struct {
	Items map[string]benchFlat
}

type benchNoTransform struct {
	A string `json:"a"`
	B string `json:"b"`
	C string `json:"c"`
}

func newBenchTransformer() *transform.Transformer {
	t := transform.New()
	t.RegisterString("upper", strings.ToUpper)
	t.RegisterString("mask", func(s string) string {
		if len(s) <= 4 {
			return ""
		}
		return s[:2] + "****" + s[len(s)-2:]
	})
	return t
}

// ========== Flat struct ==========

func BenchmarkTransform_FlatString(b *testing.B) {
	tx := newBenchTransformer()
	// Warm cache
	warm := benchFlat{Email: "x@x.com", Secret: "abcdefgh"}
	_ = tx.Transform(&warm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := benchFlat{Email: "alice@example.com", Secret: "12345678", Name: "alice", Phone: "123", Amount: 100}
		_ = tx.Transform(&v)
	}
}

// ========== Nested struct ==========

func BenchmarkTransform_Nested(b *testing.B) {
	tx := newBenchTransformer()
	warm := benchNested{Inner: benchFlat{Email: "x@x.com", Secret: "12345678"}, ID: "1"}
	_ = tx.Transform(&warm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := benchNested{Inner: benchFlat{Email: "alice@example.com", Secret: "12345678"}, ID: "keep"}
		_ = tx.Transform(&v)
	}
}

// ========== Deep nested ==========

func BenchmarkTransform_DeepNested(b *testing.B) {
	tx := newBenchTransformer()
	warm := benchDeepL1{L1: benchDeepL2{L3: benchDeepL3{Secret: "hello"}}}
	_ = tx.Transform(&warm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := benchDeepL1{L1: benchDeepL2{L3: benchDeepL3{Secret: "hello"}}}
		_ = tx.Transform(&v)
	}
}

// ========== Slice of 10 ==========

func BenchmarkTransform_Slice10(b *testing.B) {
	tx := newBenchTransformer()
	warm := benchSlice{Items: make([]benchFlat, 1)}
	warm.Items[0] = benchFlat{Email: "x@x.com"}
	_ = tx.Transform(&warm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := benchSlice{Items: make([]benchFlat, 10)}
		for j := range v.Items {
			v.Items[j] = benchFlat{Email: "alice@example.com", Secret: "12345678"}
		}
		_ = tx.Transform(&v)
	}
}

// ========== Map of 4 ==========

func BenchmarkTransform_Map4(b *testing.B) {
	tx := newBenchTransformer()
	warm := benchMap{Items: map[string]benchFlat{"x": {Email: "x@x.com"}}}
	_ = tx.Transform(&warm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := benchMap{Items: map[string]benchFlat{
			"a": {Email: "a@a.com", Secret: "12345678"},
			"b": {Email: "b@b.com", Secret: "87654321"},
			"c": {Email: "c@c.com", Secret: "11223344"},
			"d": {Email: "d@d.com", Secret: "55667788"},
		}}
		_ = tx.Transform(&v)
	}
}

// ========== No transform ==========

func BenchmarkTransform_NoTransform(b *testing.B) {
	tx := newBenchTransformer()
	warm := benchNoTransform{A: "hello", B: "world", C: "!"}
	_ = tx.Transform(&warm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := benchNoTransform{A: "hello", B: "world", C: "!"}
		_ = tx.Transform(&v)
	}
}

// ========== Cold cache ==========

func BenchmarkTransform_ColdCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tx := newBenchTransformer()
		v := benchFlat{Email: "alice@example.com", Secret: "12345678"}
		_ = tx.Transform(&v)
	}
}

// ========== Warm cache ==========

func BenchmarkTransform_WarmCache(b *testing.B) {
	tx := newBenchTransformer()
	v := benchFlat{Email: "alice@example.com", Secret: "12345678"}
	_ = tx.Transform(&v)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Email = "alice@example.com"
		v.Secret = "12345678"
		_ = tx.Transform(&v)
	}
}

// ========== Concurrent ==========

func BenchmarkTransform_Concurrent(b *testing.B) {
	tx := newBenchTransformer()
	warm := benchFlat{Email: "x@x.com", Secret: "12345678"}
	_ = tx.Transform(&warm)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v := benchFlat{Email: "alice@example.com", Secret: "12345678"}
			_ = tx.Transform(&v)
		}
	})
}

// ========== Naive baseline (per-call reflection, no cache) ==========

func naiveFns() map[string]func(string) string {
	return map[string]func(string) string{
		"upper": strings.ToUpper,
		"mask": func(s string) string {
			if len(s) <= 4 {
				return ""
			}
			return s[:2] + "****" + s[len(s)-2:]
		},
	}
}

func BenchmarkNaive_FlatString(b *testing.B) {
	fns := naiveFns()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := benchFlat{Email: "alice@example.com", Secret: "12345678", Name: "alice", Phone: "123", Amount: 100}
		naiveReflectTransform(reflect.ValueOf(&v).Elem(), fns)
	}
}

// ========== Multi-field comparison (cached vs naive) ==========

// bench10Field is a 10-field struct with 4 transformed fields.
type bench10Field struct {
	F01 string `json:"f01" transform:"upper"`
	F02 string `json:"f02"`
	F03 string `json:"f03" transform:"mask"`
	F04 string `json:"f04"`
	F05 string `json:"f05" transform:"upper"`
	F06 string `json:"f06"`
	F07 string `json:"f07" transform:"mask"`
	F08 string `json:"f08"`
	F09 string `json:"f09"`
	F10 string `json:"f10"`
}

// bench20Field is a 20-field struct with 8 transformed fields.
type bench20Field struct {
	F01 string `json:"f01" transform:"upper"`
	F02 string `json:"f02"`
	F03 string `json:"f03" transform:"mask"`
	F04 string `json:"f04"`
	F05 string `json:"f05" transform:"upper"`
	F06 string `json:"f06"`
	F07 string `json:"f07" transform:"mask"`
	F08 string `json:"f08"`
	F09 string `json:"f09" transform:"upper"`
	F10 string `json:"f10"`
	F11 string `json:"f11" transform:"mask"`
	F12 string `json:"f12"`
	F13 string `json:"f13" transform:"upper"`
	F14 string `json:"f14"`
	F15 string `json:"f15" transform:"mask"`
	F16 string `json:"f16"`
	F17 string `json:"f17"`
	F18 string `json:"f18"`
	F19 string `json:"f19"`
	F20 string `json:"f20"`
}

// bench50Field is a 50-field struct with 20 transformed fields.
type bench50Field struct {
	N01, N02, N03, N04, N05, N06, N07, N08, N09, N10 string
	P01                                              string `json:"p01" transform:"upper"`
	P02                                              string `json:"p02" transform:"mask"`
	P03                                              string `json:"p03" transform:"upper"`
	P04                                              string `json:"p04" transform:"mask"`
	P05                                              string `json:"p05" transform:"upper"`
	P06                                              string `json:"p06" transform:"mask"`
	P07                                              string `json:"p07" transform:"upper"`
	P08                                              string `json:"p08" transform:"mask"`
	P09                                              string `json:"p09" transform:"upper"`
	P10                                              string `json:"p10" transform:"mask"`
	P11                                              string `json:"p11" transform:"upper"`
	P12                                              string `json:"p12" transform:"mask"`
	P13                                              string `json:"p13" transform:"upper"`
	P14                                              string `json:"p14" transform:"mask"`
	P15                                              string `json:"p15" transform:"upper"`
	P16                                              string `json:"p16" transform:"mask"`
	P17                                              string `json:"p17" transform:"upper"`
	P18                                              string `json:"p18" transform:"mask"`
	P19                                              string `json:"p19" transform:"upper"`
	P20                                              string `json:"p20" transform:"mask"`
	N11, N12, N13, N14, N15, N16, N17, N18, N19, N20 string
}

func BenchmarkTransform_10Field(b *testing.B) {
	tx := newBenchTransformer()
	warm := bench10Field{}
	_ = tx.Transform(&warm)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := bench10Field{F01: "a", F03: "b", F05: "c", F07: "d"}
		_ = tx.Transform(&v)
	}
}

func BenchmarkNaive_10Field(b *testing.B) {
	upper := strings.ToUpper
	mask := func(s string) string {
		if len(s) <= 4 {
			return "****"
		}
		return s[:2] + "****" + s[len(s)-2:]
	}
	fns := map[string]func(string) string{"upper": upper, "mask": mask}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := bench10Field{F01: "a", F03: "b", F05: "c", F07: "d"}
		naiveReflectTransform(reflect.ValueOf(&v).Elem(), fns)
	}
}

func BenchmarkTransform_20Field(b *testing.B) {
	tx := newBenchTransformer()
	warm := bench20Field{}
	_ = tx.Transform(&warm)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := bench20Field{
			F01: "a", F03: "b", F05: "c", F07: "d",
			F09: "e", F11: "f", F13: "g", F15: "h",
		}
		_ = tx.Transform(&v)
	}
}

func BenchmarkNaive_20Field(b *testing.B) {
	upper := strings.ToUpper
	mask := func(s string) string {
		if len(s) <= 4 {
			return "****"
		}
		return s[:2] + "****" + s[len(s)-2:]
	}
	fns := map[string]func(string) string{"upper": upper, "mask": mask}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := bench20Field{
			F01: "a", F03: "b", F05: "c", F07: "d",
			F09: "e", F11: "f", F13: "g", F15: "h",
		}
		naiveReflectTransform(reflect.ValueOf(&v).Elem(), fns)
	}
}

func BenchmarkTransform_50Field(b *testing.B) {
	tx := newBenchTransformer()
	warm := bench50Field{}
	_ = tx.Transform(&warm)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := bench50Field{
			P01: "a", P02: "b", P03: "c", P04: "d",
			P05: "e", P06: "f", P07: "g", P08: "h",
			P09: "i", P10: "j", P11: "k", P12: "l",
			P13: "m", P14: "n", P15: "o", P16: "p",
			P17: "q", P18: "r", P19: "s", P20: "t",
		}
		_ = tx.Transform(&v)
	}
}

func BenchmarkNaive_50Field(b *testing.B) {
	upper := strings.ToUpper
	mask := func(s string) string {
		if len(s) <= 4 {
			return "****"
		}
		return s[:2] + "****" + s[len(s)-2:]
	}
	fns := map[string]func(string) string{"upper": upper, "mask": mask}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := bench50Field{
			P01: "a", P02: "b", P03: "c", P04: "d",
			P05: "e", P06: "f", P07: "g", P08: "h",
			P09: "i", P10: "j", P11: "k", P12: "l",
			P13: "m", P14: "n", P15: "o", P16: "p",
			P17: "q", P18: "r", P19: "s", P20: "t",
		}
		naiveReflectTransform(reflect.ValueOf(&v).Elem(), fns)
	}
}

func BenchmarkNaive_Nested(b *testing.B) {
	fns := naiveFns()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := benchNested{Inner: benchFlat{Email: "alice@example.com", Secret: "12345678"}, ID: "keep"}
		naiveReflectTransform(reflect.ValueOf(&v).Elem(), fns)
	}
}

// ========== encoding/json reference ==========
//
// json.Unmarshal is what people actually reach for when sanitizing data:
// decode into a fresh struct with a cleaning step on top. It allocates a
// new struct per call and decodes every field, so it is not an
// apples-to-apples comparison — but it is the honest one for "what does
// the standard library cost for this job".

func BenchmarkJSON_Unmarshal_Flat(b *testing.B) {
	data := []byte(`{"name":"alice","email":"alice@example.com","secret":"12345678","phone":"123","amount":100}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v benchFlat
		_ = json.Unmarshal(data, &v)
	}
}

func BenchmarkJSON_Unmarshal_20Field(b *testing.B) {
	data := []byte(`{"f01":"a","f02":"","f03":"b","f04":"","f05":"c","f06":"","f07":"d","f08":"","f09":"e","f10":"","f11":"f","f12":"","f13":"g","f14":"","f15":"h","f16":"","f17":"","f18":"","f19":"","f20":""}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v bench20Field
		_ = json.Unmarshal(data, &v)
	}
}

func BenchmarkJSON_Unmarshal_Nested(b *testing.B) {
	data := []byte(`{"inner":{"name":"alice","email":"alice@example.com","secret":"12345678","phone":"123","amount":100},"id":"keep"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v benchNested
		_ = json.Unmarshal(data, &v)
	}
}

func naiveReflectTransform(val reflect.Value, fns map[string]func(string) string) {
	for val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		f := val.Field(i)
		key := sf.Tag.Get("transform")
		if key != "" {
			if fn, ok := fns[key]; ok {
				for f.Kind() == reflect.Ptr {
					if f.IsNil() {
						break
					}
					f = f.Elem()
				}
				if f.Kind() == reflect.String && f.CanSet() {
					f.SetString(fn(f.String()))
				}
			}
		}
		// Recurse
		for f.Kind() == reflect.Ptr {
			if f.IsNil() {
				break
			}
			f = f.Elem()
		}
		switch f.Kind() {
		case reflect.Struct:
			naiveReflectTransform(f, fns)
		case reflect.Slice, reflect.Array:
			for j := 0; j < f.Len(); j++ {
				naiveReflectTransform(f.Index(j), fns)
			}
		case reflect.Map:
			iter := f.MapRange()
			for iter.Next() {
				naiveReflectTransform(iter.Value(), fns)
			}
		}
	}
}

// ========== Package vs hand-written loop (same work) ==========
//
// The library's worst case: a small struct, where the fixed reflection
// overhead is largest relative to the actual transform work. The manual
// baseline applies the exact same functions to the exact same fields, so
// the gap is purely the framework. (Wrapping either in a method changes
// nothing — the call inlines.)

type benchNorm struct {
	Email string `json:"email" transform:"normalize"`
	Name  string `json:"name" transform:"trim"`
}

func normalizeBench(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func trimBench(s string) string      { return strings.TrimSpace(s) }

// benchSink keeps results alive so the compiler cannot eliminate the field
// writes in either benchmark.
var benchSink string

func BenchmarkTransform_PackageNorm(b *testing.B) {
	tx := transform.New()
	tx.RegisterString("normalize", normalizeBench)
	tx.RegisterString("trim", trimBench)
	// Warm the cache: this measures steady state, not first-call cost.
	_ = tx.Transform(&benchNorm{Email: "  alice@example.com ", Name: " bob "})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u := benchNorm{Email: "  alice@example.com ", Name: " bob "}
		_ = tx.Transform(&u)
		benchSink = u.Email
	}
}

func BenchmarkTransform_ManualNorm(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u := benchNorm{Email: "  alice@example.com ", Name: " bob "}
		u.Email = normalizeBench(u.Email)
		u.Name = trimBench(u.Name)
		benchSink = u.Email
	}
}

// Manual version of the existing 50-field package benchmark: the same
// 20 transforms applied by hand. This is the other end of the spectrum,
// where the hand-written loop is long and the library's fixed overhead
// matters less.
func BenchmarkTransform_Manual50Field(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := bench50Field{
			P01: "a", P02: "b", P03: "c", P04: "d",
			P05: "e", P06: "f", P07: "g", P08: "h",
			P09: "i", P10: "j", P11: "k", P12: "l",
			P13: "m", P14: "n", P15: "o", P16: "p",
			P17: "q", P18: "r", P19: "s", P20: "t",
		}
		v.P01 = strings.ToUpper(v.P01)
		v.P02 = benchMaskBench(v.P02)
		v.P03 = strings.ToUpper(v.P03)
		v.P04 = benchMaskBench(v.P04)
		v.P05 = strings.ToUpper(v.P05)
		v.P06 = benchMaskBench(v.P06)
		v.P07 = strings.ToUpper(v.P07)
		v.P08 = benchMaskBench(v.P08)
		v.P09 = strings.ToUpper(v.P09)
		v.P10 = benchMaskBench(v.P10)
		v.P11 = strings.ToUpper(v.P11)
		v.P12 = benchMaskBench(v.P12)
		v.P13 = strings.ToUpper(v.P13)
		v.P14 = benchMaskBench(v.P14)
		v.P15 = strings.ToUpper(v.P15)
		v.P16 = benchMaskBench(v.P16)
		v.P17 = strings.ToUpper(v.P17)
		v.P18 = benchMaskBench(v.P18)
		v.P19 = strings.ToUpper(v.P19)
		v.P20 = benchMaskBench(v.P20)
		benchSink = v.P01
	}
}

func benchMaskBench(s string) string {
	if len(s) <= 4 {
		return ""
	}
	return s[:2] + "****" + s[len(s)-2:]
}

// ========== Realistic workloads (framework overhead ≈ noise) ==========
//
// The gap above comes from measuring trivial transforms on tiny strings:
// the fixed framework cost dominates. In realistic use — cleaning
// user-supplied text — the transform functions themselves cost
// hundreds of nanoseconds, and the framework adds only a small fraction.

func emailBench(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// collapseBench normalizes every whitespace run to a single space.
func collapseBench(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// slugBench lowercases and replaces spaces with dashes.
func slugBench(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
}

type benchReal struct {
	Email string `transform:"email"`
	Bio   string `transform:"collapse"`
	Title string `transform:"slug"`
}

var (
	realEmail = "  Alice.Smith+Spam@Example.COM "
	realBio   = "I'm a backend engineer who   loves Go, tea,   and long walks. " +
		"I write about   performance and  distributed systems,  and I think " +
		"small tools   with honest  documentation  are the best kind of  software."
	realTitle = "  My Go   Performance Notes "
)

func BenchmarkTransform_PackageReal(b *testing.B) {
	tx := transform.New()
	tx.RegisterString("email", emailBench)
	tx.RegisterString("collapse", collapseBench)
	tx.RegisterString("slug", slugBench)
	_ = tx.Transform(&benchReal{Email: realEmail, Bio: realBio, Title: realTitle})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u := benchReal{Email: realEmail, Bio: realBio, Title: realTitle}
		_ = tx.Transform(&u)
		benchSink = u.Email
	}
}

func BenchmarkTransform_ManualReal(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u := benchReal{Email: realEmail, Bio: realBio, Title: realTitle}
		u.Email = emailBench(u.Email)
		u.Bio = collapseBench(u.Bio)
		u.Title = slugBench(u.Title)
		benchSink = u.Email
	}
}

// Large text: the same simple transforms on big inputs. String work is
// linear in input size, the framework cost is fixed, so the gap shrinks
// further.
var largeText = strings.Repeat("the quick brown fox jumps over the lazy dog  ", 200)

type benchLarge struct {
	A string `transform:"upper"`
	B string `transform:"trim"`
}

func BenchmarkTransform_PackageLarge(b *testing.B) {
	tx := transform.New()
	tx.RegisterString("upper", strings.ToUpper)
	tx.RegisterString("trim", strings.TrimSpace)
	_ = tx.Transform(&benchLarge{A: largeText, B: largeText})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u := benchLarge{A: largeText, B: largeText}
		_ = tx.Transform(&u)
		benchSink = u.A
	}
}

func BenchmarkTransform_ManualLarge(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u := benchLarge{A: largeText, B: largeText}
		u.A = strings.ToUpper(u.A)
		u.B = strings.TrimSpace(u.B)
		benchSink = u.A
	}
}
