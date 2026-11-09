package addon

import (
	"reflect"
	"testing"
)

func TestParseLink(t *testing.T) {
	tests := []struct {
		name, link string
		want       Proxy
		param      [2]string
		wantErr    bool
	}{
		{name: "vless reality", link: "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=reality&sni=a.b&pbk=KEY#%E2%AD%90%20Extra",
			want:  Proxy{Scheme: "vless", Name: "⭐ Extra", Server: "example.com", Port: 443, Secret: "11111111-1111-1111-1111-111111111111"},
			param: [2]string{"security", "reality"}},
		{name: "hysteria2", link: "hysteria2://pass@h.example:8443?sni=s&obfs=salamander#hy",
			want:  Proxy{Scheme: "hysteria2", Name: "hy", Server: "h.example", Port: 8443, Secret: "pass"},
			param: [2]string{"obfs", "salamander"}},
		{name: "hy2 alias", link: "hy2://pw@h:1#n", want: Proxy{Scheme: "hysteria2", Name: "n", Server: "h", Port: 1, Secret: "pw"}},
		{name: "ipv6", link: "vless://id@[2001:db8::1]:443", want: Proxy{Scheme: "vless", Server: "2001:db8::1", Port: 443, Secret: "id"}},
		{name: "no fragment", link: "vless://id@h:443?type=ws", want: Proxy{Scheme: "vless", Server: "h", Port: 443, Secret: "id"}, param: [2]string{"type", "ws"}},
		{name: "wrong scheme", link: "ss://abc@h:1#x", wantErr: true},
		{name: "no port", link: "vless://id@h#x", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLink(tc.link)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tc.wantErr {
				return
			}
			p := got
			p.Params = nil
			if !reflect.DeepEqual(p, tc.want) {
				t.Fatalf("got %+v want %+v", p, tc.want)
			}
			if tc.param[0] != "" && got.Params.Get(tc.param[0]) != tc.param[1] {
				t.Fatalf("param %s = %q", tc.param[0], got.Params.Get(tc.param[0]))
			}
		})
	}
}
