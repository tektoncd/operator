// Code generated by cuelang.org/go/pkg/gen. DO NOT EDIT.

package sha512

import (
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/pkg/internal"
)

func init() {
	internal.Register("crypto/sha512", pkg)
}

var _ = adt.TopKind // in case the adt package isn't used

var pkg = &internal.Package{
	Native: []*internal.Builtin{{
		Name:  "Size",
		Const: "64",
	}, {
		Name:  "Size224",
		Const: "28",
	}, {
		Name:  "Size256",
		Const: "32",
	}, {
		Name:  "Size384",
		Const: "48",
	}, {
		Name:  "BlockSize",
		Const: "128",
	}, {
		Name: "Sum512",
		Params: []internal.Param{
			{Kind: adt.BytesKind | adt.StringKind},
		},
		Result: adt.BytesKind | adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			data := c.Bytes(0)
			if c.Do() {
				c.Ret = Sum512(data)
			}
		},
	}, {
		Name: "Sum384",
		Params: []internal.Param{
			{Kind: adt.BytesKind | adt.StringKind},
		},
		Result: adt.BytesKind | adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			data := c.Bytes(0)
			if c.Do() {
				c.Ret = Sum384(data)
			}
		},
	}, {
		Name: "Sum512_224",
		Params: []internal.Param{
			{Kind: adt.BytesKind | adt.StringKind},
		},
		Result: adt.BytesKind | adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			data := c.Bytes(0)
			if c.Do() {
				c.Ret = Sum512_224(data)
			}
		},
	}, {
		Name: "Sum512_256",
		Params: []internal.Param{
			{Kind: adt.BytesKind | adt.StringKind},
		},
		Result: adt.BytesKind | adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			data := c.Bytes(0)
			if c.Do() {
				c.Ret = Sum512_256(data)
			}
		},
	}},
}
