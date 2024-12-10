// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package charts

import (
	"embed"
)

var (
	// ChartSquid is the Helm chart for the squid chart.
	//go:embed squid
	ChartSquid embed.FS
	// ChartPathSquid is the path to the squid chart.
	ChartPathSquid = "squid"
)
