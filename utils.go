package main

import "strings"

func parseTags(tags string) []string {
	return strings.Split(tags, ",")
}
