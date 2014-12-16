package main

import (
	"log"
	"net/http"
)

type AlwaysRawResponseFilter struct {
	*RawResponseFilter
	Sites []string
}

func (f *AlwaysRawResponseFilter) Filter(req *http.Request, res *http.Response) (args *http.Header, err error) {
	host := res.Header.Get("Host")
	if host != "" && f.Sites != nil {
		for _, site := range f.Sites {
			// log.Printf("host %#v site %#v", host, site)
			if host == site {
				return &http.Header{
					"foo": []string{"bar"},
				}, nil
			}
		}
	}
	return nil, nil
}
