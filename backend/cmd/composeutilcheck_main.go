package main

import (
	"fmt"
	"podtainer/internal/composeutil"
)

func main() {
	content := []byte(`
services:
  web:
    image: docker.io/library/nginx:latest
    port:
      - "3000:80"
`)
	err := composeutil.Validate(content)
	fmt.Println("err:", err)

	ok := []byte(`
services:
  web:
    image: docker.io/library/nginx:latest
    ports:
      - "3000:80"
`)
	fmt.Println("err2:", composeutil.Validate(ok))
}
