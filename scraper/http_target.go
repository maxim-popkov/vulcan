// Copyright 2016 The Vulcan Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scraper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/protobuf/proto"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// HTTPTarget represents an instance of an HTTP scraper target.
type HTTPTarget struct {
	u *url.URL
	i time.Duration
	j JobName
}

// HTTPTargetConfig represents the configuration of an HTTPTarget.
type HTTPTargetConfig struct {
	Interval time.Duration
	URL      *url.URL
	JobName  JobName
}

// NewHTTPTarget creates an instance of HTTPTarget.
func NewHTTPTarget(config *HTTPTargetConfig) *HTTPTarget {
	return &HTTPTarget{
		u: config.URL,
		i: config.Interval,
		j: config.JobName,
	}
}

// Equals checkfs if the instance's current target is the same as the
// parameter other.
func (ht *HTTPTarget) Equals(other Targeter) bool {
	ot, ok := other.(*HTTPTarget)
	if !ok {
		return false
	}
	return ot.u == ht.u
}

// Fetch polls the target's metric endpoint for data and transforms it into
// a prometheus MetricFamily type.
func (ht *HTTPTarget) Fetch() ([]*dto.MetricFamily, error) {
	at := time.Now() // timestamp metrics with time scraper initiated
	fam, err := ht.fetch()
	if err != nil {
		return fam, err
	}
	timestamp(fam, at)
	return fam, nil
}

// Interval returns the current targets interval.
func (ht *HTTPTarget) Interval() time.Duration {
	return ht.i
}

// Key implements Targeter
func (ht *HTTPTarget) Key() string {
	return fmt.Sprintf("%s-%s", ht.j, ht.u)
}

func annotate(fams []*dto.MetricFamily, target Target) {
	for _, f := range fams {
		for _, m := range f.Metric {
			m.Label = append(m.Label, &dto.LabelPair{
				Name:  proto.String("job"),
				Value: proto.String(target.Job),
			})
			m.Label = append(m.Label, &dto.LabelPair{
				Name:  proto.String("instance"),
				Value: proto.String(target.Instance),
			})
		}
	}
}

func (ht HTTPTarget) fetch() ([]*dto.MetricFamily, error) {
	resp, err := http.Get(ht.u.String())
	if err != nil {
		return []*dto.MetricFamily{}, err
	}
	defer resp.Body.Close()
	// TODO check return codes
	return parse(resp.Body, resp.Header)
}

func parse(in io.Reader, header http.Header) ([]*dto.MetricFamily, error) {
	dec := expfmt.NewDecoder(in, expfmt.Negotiate(header))
	fams := []*dto.MetricFamily{}
	var err error
	for {
		var f dto.MetricFamily
		if err = dec.Decode(&f); err != nil {
			break
		}

		fams = append(fams, &f)
	}

	if err == io.EOF {
		err = nil
	}

	return fams, err
}

func timestamp(fams []*dto.MetricFamily, at time.Time) {
	timestampMs := proto.Int64(at.UnixNano() / 1e6)
	for _, f := range fams {
		for _, m := range f.Metric {
			if m.TimestampMs == nil {
				m.TimestampMs = timestampMs
			}
		}
	}
}
