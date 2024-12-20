// Copyright (c) 2024 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slack

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/projectcalico/calico/release/internal/registry"
)

var (
	//go:embed templates/success.json.gotmpl
	successMessageTemplateData string
	//go:embed templates/failure.json.gotmpl
	failureMessageTemplateData string
)

// Config is the configuration for the Slack client
type Config struct {
	// Token is the token for the Slack API
	Token string

	// Channel is the channel to post messages
	Channel string
}

func (c Config) Valid() bool {
	return c.Token != "" && c.Channel != ""
}

// MessageData is the data to be rendered in the message
type MessageData struct {
	// ReleaseName is the name of the release
	ReleaseName string

	// Product is the name of the product
	Product string

	// Stream is the stream of the release
	Stream string

	// Version is the version of the release
	Version string

	// OperatorVersion is the version of the operator
	OperatorVersion string

	// DocsURL is the URL for the release docs.
	// This is only used for success messages
	DocsURL string

	// CIURL is the URL for the CI job.
	// This is required for failure messages
	// and optional for success messages.
	CIURL string

	// ImageScanResultURL is the URL for the results from the image scanner.
	// This is only used for success messages
	ImageScanResultURL string

	// FailedImages is the list of failed images.
	// This is required for failure messages
	FailedImages []registry.Component
}

// Message is a Slack message
type Message struct {
	// Config is the configuration for the message
	Config Config

	// Data is the data to be rendered in the message
	Data MessageData
}

// Create a new Slack client
func client(token string, debug bool) *slack.Client {
	options := []slack.Option{}
	if debug {
		options = append(options, slack.OptionDebug(true))
	}
	client := slack.New(token, options...)
	return client
}

// SendFailure sends a failure message to Slack
func (m *Message) SendFailure(debug bool) error {
	if len(m.Data.FailedImages) == 0 {
		return fmt.Errorf("no failed images to report")
	}
	if m.Data.CIURL == "" {
		return fmt.Errorf("CI URL is required for failure messages")
	}
	client := client(m.Config.Token, debug)
	return m.send(client, failureMessageTemplateData)
}

// SendSuccess sends a success message to Slack
func (m *Message) SendSuccess(debug bool) error {
	client := client(m.Config.Token, debug)
	return m.send(client, successMessageTemplateData)
}

// send sends the message to Slack
func (m *Message) send(client *slack.Client, messageTemplateData string) error {
	message, err := m.renderMessage(messageTemplateData)
	if err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"channel": m.Config.Channel,
		"message": message,
	}).Debug("Sending message to Slack")
	_, _, err = client.PostMessage(m.Config.Channel, slack.MsgOptionBlocks(message...))
	return err
}

// renderMessage renders the message from the template
func (m *Message) renderMessage(templateData string) ([]slack.Block, error) {
	tmpl, err := template.New("message").Parse(templateData)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, m.Data); err != nil {
		return nil, err
	}
	blocks := slack.Blocks{}
	if err := blocks.UnmarshalJSON(buf.Bytes()); err != nil {
		return nil, err
	}
	return blocks.BlockSet, nil
}
