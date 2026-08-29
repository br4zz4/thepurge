package main

import (
    "os"
    "strings"
    "time"
)

func main(){
    b, err := os.ReadFile("/etc/hostname")
    println("read bytes:", len(b), "err:", err)
    s := string(b)
    println("has content:", len(s) > 0)
    parts := strings.Split("a,b,c", ",")
    println("split:", len(parts))
    t := time.Now()
    println("now nsec:", t.UnixNano())
    os.WriteFile("/tmp/tp_probe_out.txt", []byte("hello\n"), 0644)
    b2, err := os.ReadFile("/tmp/tp_probe_out.txt")
    println("write+rebytes:", len(b2), "err:", err, "str:", string(b2))
}
