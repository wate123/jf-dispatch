package main

import "os"

func getEnv(k string) (string, bool) { return os.LookupEnv(k) }
