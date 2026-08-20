package mdtotex

import "sort"

// Carrier is one chapter that claims an identifier, and the heading behind it.
type Carrier struct {
	Chapter string
	Heading string
}

// Collision is one identifier claimed by more than one chapter.
//
// LaTeX reports this as a duplicate label pointing at whichever chapter came
// second in the container's input list, which names neither the chapter that
// caused it nor the heading (srd002-renderer-core R7.3).
type Collision struct {
	Identifier string
	Carriers   []Carrier
}

// Collisions returns every identifier that appears in more than one of the
// given results, naming each chapter that carries it and the heading behind
// it (srd002-renderer-core R7.3).
//
// It is a function over the reports conversion returned, not over fragments or
// files: it opens nothing, so a caller holding conversions in memory calls it
// without writing them out first (R7.4). It returns the collisions rather than
// an error, because a manuscript mid-edit may carry one the author is already
// fixing, and a library that refuses to proceed is one the caller works around
// (R7.6).
//
// An identifier the author stated in two chapters is reported exactly as two
// derived ones are. Both break the same compile (R7.5).
//
// Collisions within a single chapter do not reach here: conversion fails on
// them, naming both headings (srd002-renderer-core R3.6).
func Collisions(results ...Result) []Collision {
	carriers := make(map[string][]Carrier)
	order := make([]string, 0, len(results))

	for _, result := range results {
		for _, label := range result.Labels {
			if _, seen := carriers[label.Identifier]; !seen {
				order = append(order, label.Identifier)
			}
			carriers[label.Identifier] = append(carriers[label.Identifier], Carrier{
				Chapter: result.Name,
				Heading: label.Heading,
			})
		}
	}

	var collisions []Collision
	for _, identifier := range order {
		if len(carriers[identifier]) > 1 {
			collisions = append(collisions, Collision{
				Identifier: identifier,
				Carriers:   carriers[identifier],
			})
		}
	}

	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i].Identifier < collisions[j].Identifier
	})
	return collisions
}
