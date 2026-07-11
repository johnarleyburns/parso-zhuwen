package images

import "testing"

func TestCommonsTitleFromSource(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"plain", "https://commons.wikimedia.org/wiki/File:Rabbit.jpg", "File:Rabbit.jpg"},
		{"not-commons", "https://example.com/wiki/File:Rabbit.jpg", ""},
		{"query-stripped", "https://commons.wikimedia.org/wiki/File:Rabbit.jpg?uselang=en", "File:Rabbit.jpg"},
		// Percent-encoded titles must decode to the API title form, else the
		// thumbnail fetch silently misses and the cover falls back to a stub.
		{"cjk", "https://commons.wikimedia.org/wiki/File:%E5%85%AB%E4%BB%99%E8%BF%87%E6%B5%B7.jpg", "File:八仙过海.jpg"},
		{"apostrophe", "https://commons.wikimedia.org/wiki/File:Chang%27e_flees_to_the_moon.jpg", "File:Chang'e_flees_to_the_moon.jpg"},
		{"accent", "https://commons.wikimedia.org/wiki/File:Mosa%C3%AFcanada_2017.jpg", "File:Mosaïcanada_2017.jpg"},
		{"ampersand", "https://commons.wikimedia.org/wiki/File:Ice_%26_sun.jpg", "File:Ice_&_sun.jpg"},
		{"right-single-quote", "https://commons.wikimedia.org/wiki/File:Farmer%E2%80%99s_Wife.jpg", "File:Farmer\u2019s_Wife.jpg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CommonsTitleFromSource(c.url); got != c.want {
				t.Fatalf("CommonsTitleFromSource(%q) = %q, want %q", c.url, got, c.want)
			}
		})
	}
}
