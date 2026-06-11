// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package lite_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLiteParser(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Parser Lite Suite")
}
