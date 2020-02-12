// snippet-comment:[These are tags for the AWS doc team's sample catalog. Do not remove.]
// snippet-sourceauthor:[Doug-AWS]
// snippet-sourcedescription:[Creates a CloudFormation stack.]
// snippet-keyword:[AWS CloudFormation]
// snippet-keyword:[CreateStack function]
// snippet-keyword:[WaitUntilStackCreateComplete function]
// snippet-keyword:[Go]
// snippet-sourcesyntax:[go]
// snippet-service:[cloudformation]
// snippet-keyword:[Code Sample]
// snippet-sourcetype:[full-example]
// snippet-sourcedate:[2018-03-16]
/*
   Copyright 2010-2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.

   This file is licensed under the Apache License, Version 2.0 (the "License").
   You may not use this file except in compliance with the License. A copy of
   the License is located at

    http://aws.amazon.com/apache2.0/

   This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
   CONDITIONS OF ANY KIND, either express or implied. See the License for the
   specific language governing permissions and limitations under the License.
*/

package main

import (
    "encoding/json"
    "errors"
    "fmt"
    "io/ioutil"
    "log"
    "strconv"
    "testing"
    "time"

    "github.com/google/uuid"

    "github.com/aws/aws-sdk-go/aws/session"
)

// Config stores our global configuration values (replace env values with these)
type Config struct {
    MaxRetrySeconds int    `json:"MaxRetrySeconds"`
    TemplateFile    string `json:"TemplateFile"`
    Debug           bool   `json:"Debug"`
}

// Global value for name of configuration file
var ConfigFileName = "config.json"

// Gloval variable for configuration set in config.json
var GlobalConfig Config

// PopulateConfiguration fills in the values from config.json
func PopulateConfiguration() error {
    // Get configuration from config.json

    // Get entire file as a JSON string
    content, err := ioutil.ReadFile(ConfigFileName)
    if err != nil {
        return err
    }

    // Convert []byte to string
    text := string(content)

    // Marshall JSON string in text into global struct
    json.Unmarshal([]byte(text), &GlobalConfig)

    return nil
}

func GetTemplateFromFile(filename string) (string, error) {
    // Get template from file
    content, err := ioutil.ReadFile(filename)
    if err != nil {
        fmt.Println("Could not read template file " + GlobalConfig.TemplateFile)
        fmt.Println(err)
        return "", err
    }

    // Convert []byte to string
    templateBody := string(content)
    return templateBody, nil
}

func DebugPrint(t *testing.T, debug bool, s string) {
    if debug {
        t.Log(s)
    }
}

func TestCfnCrudOps(t *testing.T) {
    // Get config values from config.json
    err := PopulateConfiguration()
    if err != nil {
        t.Errorf("Could not load configuration from %s", ConfigFileName)
        return
    }

    t.Log("Debugging:       Enabled")
    t.Log("MaxRetrySeconds: " + strconv.Itoa(int(GlobalConfig.MaxRetrySeconds)))
    t.Log("TemplateFile:    " + GlobalConfig.TemplateFile)

    // Make sure we have a value for template file
    if GlobalConfig.TemplateFile == "" {
        t.Errorf("Configuration is missing value for TemplateFile")
        return
    }

    // Get template from file
    templateBody, err := GetTemplateFromFile(GlobalConfig.TemplateFile)

    // Initialize a session that the SDK uses to load
    // credentials from the shared credentials file ~/.aws/credentials
    // and configuration from the shared configuration file ~/.aws/config.
    sess := session.Must(session.NewSessionWithOptions(session.Options{
        SharedConfigState: session.SharedConfigEnable,
    }))

    // Create stack using random name
    id := uuid.New()
    stackName := "stack-" + id.String()

    DebugPrint(t, GlobalConfig.Debug, "Creating stack "+stackName)
    err = CreateStack(sess, stackName, templateBody)
    if err != nil {
        t.Errorf("Got error %s trying to create stack %s", err.Error(), stackName)
        return
    }

    t.Log("Created stack " + stackName)

    nameInList := false
    var name NameAndStatus

    DebugPrint(t, GlobalConfig.Debug, "Looking for "+stackName+" in list of stacks")

    // Wait 10, 20, 40 seconds, ... until we've waited a maximum of MaxRetrySeconds (is there a better way???)
    var duration int64
    duration = 1
    ts := MultiplyDuration(duration, time.Second)

    for int(duration) < GlobalConfig.MaxRetrySeconds {
        t.Log("Sleeping " + strconv.Itoa(int(duration)) + " seconds")
        time.Sleep(ts)

        // Get list of stacks
        stacks, err := GetStackNamesAndStatus(sess)
        if err != nil {
            log.Fatal(err)
            return
        }

        DebugPrint(t, GlobalConfig.Debug, ".")
        name, nameInList = IsNameInList(stacks, stackName)
        if nameInList {
            DebugPrint(t, GlobalConfig.Debug, "Found "+stackName+" in list of stacks")
            break
        }

        duration = duration * 2
    }

    if !nameInList {
        err := errors.New("Could not find " + stackName + " in list of stacks")
        t.Fatal(err)
    }

    // Now delete the stack
    DebugPrint(t, GlobalConfig.Debug, "Deleting stack "+stackName)
    err = DeleteStack(sess, stackName)
    if err != nil {
        t.Fatal(err)
    }

    DebugPrint(t, GlobalConfig.Debug, "Looking for "+stackName+" in list of stacks")
    duration = 1
    ts = MultiplyDuration(duration, time.Second)
    status := ""

    for int(duration) < GlobalConfig.MaxRetrySeconds {
        t.Log("Sleeping " + strconv.Itoa(int(duration)) + " seconds")
        time.Sleep(ts)

        stacks, err := GetStackNamesAndStatus(sess)
        if err != nil {
            log.Fatal(err)
            return
        }

        // Stack should be in list as DELETE_COMPLETED
        DebugPrint(t, GlobalConfig.Debug, ".")
        name, nameInList = IsNameInList(stacks, stackName)

        status = string(name[1])

        if nameInList && status == "DELETE_COMPLETE" {
            break
        }

        duration = duration * 2
    }

    if nameInList && status == "DELETE_COMPLETE" {
        t.Log("Found " + name[0] + ", with status DELETE_COMPLETE, as expected")
    } else {
        msg := name[0] + " had unexpected status: " + status
        t.Fatal(msg)
    }
}
