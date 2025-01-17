/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tap

import (
	"bufio"
	"encoding/json"
	"github.com/apache/incubator-devlake/config"
	"github.com/apache/incubator-devlake/errors"
	"github.com/apache/incubator-devlake/utils"
	"github.com/mitchellh/hashstructure"
	"os"
	"os/exec"
	"path/filepath"
)

const singerPropertiesDir = "SINGER_PROPERTIES_DIR"

type (
	// SingerTap the Singer implementation of Tap
	SingerTap struct {
		*SingerTapConfig
		cmd            string
		name           string
		tempLocation   string
		propertiesFile *fileData[SingerTapProperties]
		stateFile      *fileData[[]byte]
		configFile     *fileData[[]byte]
	}
	fileData[Content any] struct {
		path    string
		content *Content
	}
)

// NewSingerTap the constructor for SingerTap
func NewSingerTap(cfg *SingerTapConfig) (*SingerTap, errors.Error) {
	tempDir, err := errors.Convert01(os.MkdirTemp("", "singer"+"_*"))
	if err != nil {
		return nil, errors.Default.Wrap(err, "couldn't create temp directory for singer-tap")
	}
	propsFile, err := readProperties(tempDir, cfg)
	if err != nil {
		return nil, err
	}
	tapName := filepath.Base(cfg.Cmd)
	return &SingerTap{
		cmd:             cfg.Cmd,
		name:            tapName,
		tempLocation:    tempDir,
		propertiesFile:  propsFile,
		SingerTapConfig: cfg,
	}, nil
}

// SetConfig implements Tap.SetConfig
func (t *SingerTap) SetConfig(cfg any) errors.Error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return errors.Default.Wrap(err, "error reading singer-tap mappings")
	}
	file, err := os.OpenFile(filepath.Join(t.tempLocation, "config.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return errors.Default.Wrap(err, "error opening singer-tap config file")
	}
	_, err = file.Write(b)
	if err != nil {
		return errors.Default.Wrap(err, "error writing to singer-tap config file")
	}
	t.configFile = &fileData[[]byte]{
		path:    file.Name(),
		content: &b,
	}
	return nil
}

// SetState implements Tap.SetState
func (t *SingerTap) SetState(state any) errors.Error {
	b, err := json.Marshal(state)
	if err != nil {
		return errors.Default.Wrap(err, "error serializing singer-tap state")
	}
	file, err := os.OpenFile(filepath.Join(t.tempLocation, "state.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return errors.Default.Wrap(err, "error opening singer-tap state file")
	}
	_, err = file.Write(b)
	if err != nil {
		return errors.Default.Wrap(err, "error writing to singer-tap state file")
	}
	t.stateFile = &fileData[[]byte]{
		path:    file.Name(),
		content: &b,
	}
	return nil
}

// SetProperties implements Tap.SetProperties
func (t *SingerTap) SetProperties(streamName string, propsModifier func(props *SingerTapStream) bool) (uint64, errors.Error) {
	modified := t.modifyProperties(streamName, propsModifier)
	err := t.writeProperties()
	if err != nil {
		return 0, errors.Default.Wrap(err, "error trying to modify singer-tap properties")
	}
	return hash(modified)
}

// GetName implements Tap.GetName
func (t *SingerTap) GetName() string {
	return t.name
}

// Run implements Tap.Run
func (t *SingerTap) Run() (<-chan *utils.ProcessResponse[Output[json.RawMessage]], errors.Error) {
	catalogCmd := "--catalog"
	if t.IsLegacy {
		catalogCmd = "--properties"
	}
	args := []string{"--config", t.configFile.path, catalogCmd, t.propertiesFile.path}
	if t.stateFile != nil {
		args = append(args, []string{"--state", t.stateFile.path}...)
	}
	cmd := exec.Command(t.cmd, args...)
	stream, err := utils.StreamProcess(cmd, func(b []byte) (Output[json.RawMessage], error) {
		var output Output[json.RawMessage]
		output, err := NewSingerTapOutput(b)
		if err != nil {
			return nil, err
		}
		return output, nil //data is expected to be JSON
	})
	if err != nil {
		return nil, errors.Default.Wrap(err, "error starting process stream from singer-tap")
	}
	return stream, nil
}

func readProperties(tempDir string, cfg *SingerTapConfig) (*fileData[SingerTapProperties], errors.Error) {
	globalDir := config.GetConfig().GetString(singerPropertiesDir)
	_, err := os.Stat(globalDir)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error getting singer props directory")
	}
	globalPath := filepath.Join(globalDir, cfg.StreamPropertiesFile)
	b, err := os.ReadFile(globalPath)
	if err != nil {
		panic(err)
	}
	var props SingerTapProperties
	err = json.Unmarshal(b, &props)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error deserializing singer-tap properties")
	}
	return &fileData[SingerTapProperties]{
		path:    filepath.Join(tempDir, "properties.json"),
		content: &props,
	}, nil
}

func (t *SingerTap) writeProperties() error {
	file, err := os.OpenFile(t.propertiesFile.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	b, err := json.Marshal(t.propertiesFile.content)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	if _, err = writer.Write(b); err != nil {
		return err
	}
	return writer.Flush()
}

func hash(x any) (uint64, errors.Error) {
	version, err := hashstructure.Hash(x, nil)
	if err != nil {
		return 0, errors.Default.WrapRaw(err)
	}
	return version, nil
}

var _ Tap[SingerTapStream] = (*SingerTap)(nil)

func (t *SingerTap) modifyProperties(streamName string, propsModifier func(props *SingerTapStream) bool) *SingerTapStream {
	properties := t.propertiesFile.content
	for i := 0; i < len(properties.Streams); i++ {
		stream := properties.Streams[i]
		if !defaultSingerStreamSetter(streamName, stream) {
			continue
		}
		if propsModifier != nil && propsModifier(stream) {
			return stream
		}
	}
	return nil
}

func defaultSingerStreamSetter(name string, stream *SingerTapStream) bool {
	if stream.Stream != name {
		return false
	}
	for _, meta := range stream.Metadata {
		innerMeta := meta["metadata"].(map[string]any)
		innerMeta["selected"] = true
	}
	return true
}
