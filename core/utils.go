package core

import "net/url"

type URLParams struct {
	Key, Val string
}

func MakeURL(rawURL string, params []URLParams) (u *url.URL, err error) {
	u, err = url.Parse(rawURL)
	if err != nil {
		LogError("Failed to parse URL: ", err)
		return
	}
	q := u.Query()
	for _, p := range params {
		q.Set(p.Key, p.Val)
	}
	u.RawQuery = q.Encode()
	return
}
