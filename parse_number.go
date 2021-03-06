/*
 * MinIO Cloud Storage, (C) 2020 MinIO, Inc.
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

package simdjson

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"unsafe"
)

const (
	isPartOfNumberFlag = 1 << iota
	isFloatOnlyFlag
	isMinusFlag
	isEOVFlag
	isDigitFlag
	isMustHaveDigitNext
)

var isNumberRune = [256]uint8{
	'0':  isPartOfNumberFlag | isDigitFlag,
	'1':  isPartOfNumberFlag | isDigitFlag,
	'2':  isPartOfNumberFlag | isDigitFlag,
	'3':  isPartOfNumberFlag | isDigitFlag,
	'4':  isPartOfNumberFlag | isDigitFlag,
	'5':  isPartOfNumberFlag | isDigitFlag,
	'6':  isPartOfNumberFlag | isDigitFlag,
	'7':  isPartOfNumberFlag | isDigitFlag,
	'8':  isPartOfNumberFlag | isDigitFlag,
	'9':  isPartOfNumberFlag | isDigitFlag,
	'.':  isPartOfNumberFlag | isFloatOnlyFlag | isMustHaveDigitNext,
	'+':  isPartOfNumberFlag,
	'-':  isPartOfNumberFlag | isMinusFlag | isMustHaveDigitNext,
	'e':  isPartOfNumberFlag | isFloatOnlyFlag,
	'E':  isPartOfNumberFlag | isFloatOnlyFlag,
	',':  isEOVFlag,
	'}':  isEOVFlag,
	']':  isEOVFlag,
	' ':  isEOVFlag,
	'\t': isEOVFlag,
	'\r': isEOVFlag,
	'\n': isEOVFlag,
	':':  isEOVFlag,
}

// parseNumber will parse the number starting in the buffer.
// Any non-number characters at the end will be ignored.
// Returns TagEnd if no valid value found be found.
func parseNumber(buf []byte) (id, val uint64) {
	pos := 0
	found := uint8(0)
	for i, v := range buf {
		t := isNumberRune[v]
		if t == 0 {
			//fmt.Println("aborting on", string(v), "in", string(buf[:i]))
			return 0, 0
		}
		if t == isEOVFlag {
			break
		}
		if t&isMustHaveDigitNext > 0 {
			// A period and minus must be followed by a digit
			if len(buf) < i+2 || isNumberRune[buf[i+1]]&isDigitFlag == 0 {
				return 0, 0
			}
		}
		found |= t
		pos = i + 1
	}
	if pos == 0 {
		return 0, 0
	}
	const maxIntLen = 20
	floatTag := uint64(TagFloat) << JSONTAGOFFSET

	// Only try integers if we didn't find any float exclusive and it can fit in an integer.
	if found&isFloatOnlyFlag == 0 && pos <= maxIntLen {
		if found&isMinusFlag == 0 {
			if pos > 1 && buf[0] == '0' {
				// Integers cannot have a leading zero.
				return 0, 0
			}
		} else {
			if pos > 2 && buf[1] == '0' {
				// Integers cannot have a leading zero after minus.
				return 0, 0
			}
		}
		i64, err := strconv.ParseInt(unsafeBytesToString(buf[:pos]), 10, 64)
		if err == nil {
			return uint64(TagInteger) << JSONTAGOFFSET, uint64(i64)
		}
		if errors.Is(err, strconv.ErrRange) {
			floatTag |= uint64(FloatOverflowedInteger)
		}

		if found&isMinusFlag == 0 {
			u64, err := strconv.ParseUint(unsafeBytesToString(buf[:pos]), 10, 64)
			if err == nil {
				return uint64(TagUint) << JSONTAGOFFSET, u64
			}
			if errors.Is(err, strconv.ErrRange) {
				floatTag |= uint64(FloatOverflowedInteger)
			}
		}
	} else if found&isFloatOnlyFlag == 0 {
		floatTag |= uint64(FloatOverflowedInteger)
	}

	if pos > 1 && buf[0] == '0' && isNumberRune[buf[1]]&isFloatOnlyFlag == 0 {
		// Float can only have have a leading 0 when followed by a period.
		return 0, 0
	}
	f64, err := strconv.ParseFloat(unsafeBytesToString(buf[:pos]), 64)
	if err == nil {
		return floatTag, math.Float64bits(f64)
	}
	return 0, 0
}

// unsafeBytesToString should only be used when we have control of b.
func unsafeBytesToString(b []byte) (s string) {
	var length = len(b)

	if length == 0 {
		return ""
	}

	stringHeader := (*reflect.StringHeader)(unsafe.Pointer(&s))
	stringHeader.Data = uintptr(unsafe.Pointer(&b[0]))
	stringHeader.Len = length
	return s
}
