// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package misc

import (
	"iter"
	"slices"
)

// Map returns a map iterator on a sequence.
func Map[S, T any](seq iter.Seq[S], xform func(S) T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for s := range seq {
			if !yield(xform(s)) {
				return
			}
		}
	}
}

// Values returns an iterator over the variadic args.
func Values[S any](args ...S) iter.Seq[S] {
	return slices.Values(args)
}

// Filter returns a filtered by cond sequence of the given one.
func Filter[S any](seq iter.Seq[S], cond func(S) bool) iter.Seq[S] {
	return func(yield func(S) bool) {
		for s := range seq {
			if cond(s) && !yield(s) {
				return
			}
		}
	}
}

// ParseSeq parses the elements of a given sequence with
// a given parse function and returns a sequence of the parsing
// results that do not fail to parse.
func ParseSeq[S, T any](seq iter.Seq[S], parse func(S) (T, error)) iter.Seq[T] {
	return func(yield func(T) bool) {
		for s := range seq {
			if t, err := parse(s); err == nil {
				if !yield(t) {
					return
				}
			}
		}
	}
}

// Attribute returns a sequence attributing the given one with a given attribute.
func Attribute[S, A any](seq iter.Seq[S], a A) iter.Seq2[S, A] {
	return func(yield func(S, A) bool) {
		for s := range seq {
			if !yield(s, a) {
				return
			}
		}
	}
}

// Join2 joins a list of sequences.
func Join2[K, V any](seqs ...iter.Seq2[K, V]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, seq := range seqs {
			for k, v := range seq {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}
