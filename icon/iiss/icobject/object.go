/*
 * Copyright 2020 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package icobject

import (
	"reflect"

	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/common/db"
	"github.com/icon-project/goloop/common/errors"
	"github.com/icon-project/goloop/common/merkle"
	"github.com/icon-project/goloop/common/trie"
)

const TypeReserved int = 30

type Tag int

func (t Tag) Type() int {
	return int(t) >> 3
}

func (t Tag) Version() int {
	return int(t) & 0x7
}

func MakeTag(t int, v int) Tag {
	return Tag(t<<3 | (v & 0x7))
}

type Impl interface {
	Version() int
	RLPDecodeFields(decoder codec.Decoder) error
	RLPEncodeFields(encoder codec.Encoder) error
	Reset(dbase db.Database) error
	Resolve(builder merkle.Builder) error
	ClearCache()
	Flush() error
	Equal(o Impl) bool
}

type Object struct {
	factory ImplFactory
	bytes   []byte
	tag     Tag
	real    Impl
}

var ObjectType = reflect.TypeOf((*Object)(nil))

func (o *Object) Equal(object trie.Object) bool {
	oo := object.(*Object)
	if oo == o {
		return true
	}
	return o.real.Equal(oo.real)
}

func (o *Object) Resolve(builder merkle.Builder) error {
	return o.real.Resolve(builder)
}

func (o *Object) ClearCache() {
	o.real.ClearCache()
}

func (o *Object) Reset(dbase db.Database, bs []byte) error {
	factory := FactoryOf(dbase)
	if factory == nil {
		return errors.InvalidStateError.New("FactoryIsNotAttached")
	}
	o.factory = factory
	o.bytes = bs
	if _, err := codec.BC.UnmarshalFromBytes(bs, o); err != nil {
		return err
	}
	return o.real.Reset(dbase)
}

func (o *Object) Bytes() []byte {
	if o.bytes == nil {
		o.bytes = codec.BC.MustMarshalToBytes(o)
	}
	return o.bytes
}

func (o *Object) RLPDecodeSelf(d codec.Decoder) error {
	d2, err := d.DecodeList()
	if err != nil {
		return err
	}
	var tag Tag
	var real Impl
	if err := d2.Decode(&tag); err != nil {
		return err
	}
	real, err = o.factory(tag)
	if err != nil {
		return errors.CriticalFormatError.Wrap(err,
			"FailToCreateObjectImpl")
	}
	err = real.RLPDecodeFields(d2)
	if err != nil {
		return err
	}
	o.real = real
	o.tag = tag
	return nil
}

func (o *Object) RLPEncodeSelf(e codec.Encoder) error {
	e2, err := e.EncodeList()
	if err != nil {
		return err
	}
	if err := e2.Encode(o.tag); err != nil {
		return err
	}
	return o.real.RLPEncodeFields(e2)
}

func (o *Object) Flush() error {
	return o.real.Flush()
}

func (o *Object) Real() Impl {
	if o == nil {
		return nil
	}
	return o.real
}

func (o *Object) Tag() Tag {
	if o == nil {
		return 0
	}
	return o.tag
}

func New(t int, real Impl) *Object {
	return &Object{
		tag:  MakeTag(t, real.Version()),
		real: real,
	}
}

type NoDatabase struct{}

func (o *NoDatabase) Flush() error {
	return nil
}

func (o *NoDatabase) ClearCache() {
	// do nothing
}

func (o *NoDatabase) Reset(dbase db.Database) error {
	// do nothing
	return nil
}

func (o *NoDatabase) Resolve(bd merkle.Builder) error {
	// do nothing
	return nil
}
