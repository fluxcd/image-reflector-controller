package controllers

import "testing"

func Fuzz_imagerepository_getURLHost(f *testing.F) {
	f.Add("http://test")
	f.Add("http://")
	f.Add("http:///")
	f.Add("test")
	f.Add(" ")

	f.Fuzz(func(t *testing.T, url string) {
		_, _ = getURLHost(url)
	})
}
