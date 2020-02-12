/*
   Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

   This file is licensed under the Apache License, Version 2.0 (the "License").
   You may not use this file except in compliance with the License. A copy of
   the License is located at

    http://aws.amazon.com/apache2.0/

   This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
   CONDITIONS OF ANY KIND, either express or implied. See the License for the
   specific language governing permissions and limitations under the License.
*/
// snippet-start:[cfn.go.crud_ops.entire]
package main

import (
    "flag"
    "fmt"
    "io/ioutil"
    "time"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/cloudformation"
    "github.com/google/uuid"
)

// CreateStack creates a stack
func CreateStack(sess *session.Session, stackName string, templateBody string) error {
    svc := cloudformation.New(sess)

    input := &cloudformation.CreateStackInput{TemplateBody: aws.String(templateBody), StackName: aws.String(stackName)}

    _, err := svc.CreateStack(input)
    if err != nil {
        return err
    }

    desInput := &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)}
    err = svc.WaitUntilStackCreateComplete(desInput)
    if err != nil {
        return err
    }

    return nil
}

// NameAndStatus holds a stack name and status
type NameAndStatus [2]string

// StackNamesAndStatus is a list of stack names and their status
type StackNamesAndStatus []NameAndStatus

// GetStackNamesAndStatus gets a list of stack names and their status
func GetStackNamesAndStatus(sess *session.Session) (StackNamesAndStatus, error) {
    var nameAndStatus NameAndStatus
    var stacks StackNamesAndStatus
    // Create CloudFormation client
    svc := cloudformation.New(sess)

    // All status values
    var filter = []*string{aws.String("CREATE_IN_PROGRESS"), aws.String("CREATE_FAILED"), aws.String("CREATE_COMPLETE"), aws.String("DELETE_COMPLETE"), aws.String("ROLLBACK_IN_PROGRESS"), aws.String("ROLLBACK_FAILED"), aws.String("ROLLBACK_COMPLETE"), aws.String("DELETE_IN_PROGRESS"), aws.String("DELETE_FAILED"), aws.String("UPDATE_IN_PROGRESS"), aws.String("UPDATE_COMPLETE_CLEANUP_IN_PROGRESS"), aws.String("UPDATE_COMPLETE"), aws.String("UPDATE_ROLLBACK_IN_PROGRESS"), aws.String("UPDATE_ROLLBACK_FAILED"), aws.String("UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS"), aws.String("UPDATE_ROLLBACK_COMPLETE"), aws.String("REVIEW_IN_PROGRESS")}
    input := &cloudformation.ListStacksInput{StackStatusFilter: filter}

    resp, err := svc.ListStacks(input)
    if err != nil {
        return stacks, err
    }

    for _, stack := range resp.StackSummaries {
        nameAndStatus = NameAndStatus{*stack.StackName, *stack.StackStatus}
        stacks = append(stacks, nameAndStatus)
    }

    return stacks, nil
}

// IsNameInList determines whether name is in list
// and if so, the name and its status
func IsNameInList(list StackNamesAndStatus, name string) (NameAndStatus, bool) {
    var nameAndStatus NameAndStatus

    for _, stack := range list {
        if stack[0] == name {
            nameAndStatus = NameAndStatus{stack[0], stack[1]}
            return nameAndStatus, true
        }
    }

    return nameAndStatus, false
}

// DeleteStack deletes the stack stackName
func DeleteStack(sess *session.Session, stackName string) error {
    svc := cloudformation.New(sess)

    delInput := &cloudformation.DeleteStackInput{StackName: aws.String(stackName)}

    _, err := svc.DeleteStack(delInput)
    if err != nil {
        return err
    }

    // Wait until stack is deleted
    desInput := &cloudformation.DescribeStacksInput{StackName: aws.String("my-groovy-stack")}

    err = svc.WaitUntilStackDeleteComplete(desInput)
    if err != nil {
        return err
    }

    return nil
}

// MultiplyDuration gets a time duration, in seconds
func MultiplyDuration(factor int64, d time.Duration) time.Duration {
    return time.Duration(factor) * d
}

func main() {
    maxRetrySecondsPtr := flag.Int64("d", 100, "Max seconds to sleep before listing")
    operationPtr := flag.String("o", "all", "Whether to 'create', 'list', or 'delete' stack, or 'all' (default) to do them all")
    stackNamePtr := flag.String("n", "", "The name of the stack to create, list, or delete")
    templateFilePtr := flag.String("t", "", "The local file containing the template; required for create")

    flag.Parse()
    maxRetrySeconds := *maxRetrySecondsPtr
    operation := *operationPtr
    stackName := *stackNamePtr
    templateFile := *templateFilePtr

    if !(operation == "all" || operation == "create" || operation == "delete" || operation == "list") {
        fmt.Println("Unknown operation: " + operation)
        return
    }

    if maxRetrySeconds < 10 {
        maxRetrySeconds = 10
    }

    if (operation == "create" || operation == "all") && templateFile == "" {
        fmt.Println("You must supply the name of a template file:")
        fmt.Println("go run CfnCrudOps -t TEMPLATE-FILE")
        return
    }

    templateBody := ""

    if operation == "create" || operation == "all" {
        // Snarf template from file
        content, err := ioutil.ReadFile(templateFile)
        if err != nil {
            fmt.Println("Could not read template file " + templateFile)
            fmt.Println(err)
            return
        }

        // Convert []byte to string
        templateBody = string(content)
    }

    if stackName == "" {
        // Create random stack name
        id := uuid.New()
        stackName = "stack-" + id.String()
        fmt.Println("Created stack name " + stackName)
    }

    // Initialize a session that the SDK uses to load
    // credentials from the shared credentials file ~/.aws/credentials
    // and configuration from the shared configuration file ~/.aws/config.
    sess := session.Must(session.NewSessionWithOptions(session.Options{
        SharedConfigState: session.SharedConfigEnable,
    }))

    if operation == "create" || operation == "all" {
        // Create stack
        err := CreateStack(sess, stackName, templateBody)
        if err != nil {
            fmt.Println("Could not create stack " + stackName)
            fmt.Println(err)
            return
        }
    }

    if operation == "create" || operation == "delete" {
        var duration int64
        duration = 10
        stackInNames := false

        for duration < maxRetrySeconds {
            // Get list of stacks
            //     func GetStackNamesAndStatus(sess *session.Session) (StackNamesAndStatus, error)
            stacks, err := GetStackNamesAndStatus(sess)
            if err != nil {
                fmt.Println("Could not get stack namees")
                fmt.Println(err)
                return
            }

            _, stackInNames := IsNameInList(stacks, stackName)

            if stackInNames {
                fmt.Println("Found " + stackName + " in list of stacks, as expected")
                break
            }

            duration = duration * 2
        }

        if !stackInNames {
            fmt.Println("Could not find " + stackName + " in list of stacks")
            return
        }
    } else {
        // List operation
        // Just list stacks
        stacks, err := GetStackNamesAndStatus(sess)
        if err != nil {
            fmt.Println("Could not get stack namees")
            fmt.Println(err)
            return
        }

        for _, stack := range stacks {
            if stackName != "" && stackName == stack[0] {
                // Just list that stack
                fmt.Println(stack[0] + ", Status: " + stack[1])
            } else {
                fmt.Println(stack[0] + ", Status: " + stack[1])
            }
        }
    }

    if operation == "delete" || operation == "all" {
        // Delete the stack
        err := DeleteStack(sess, stackName)
        if err != nil {
            fmt.Println("Could not delete stack " + stackName)
            fmt.Println(err)
            return
        }

        fmt.Println(stackName + " should NOT be in the following list:")

        stacks, err := GetStackNamesAndStatus(sess)
        if err != nil {
            fmt.Println("Could not get stack namees")
            fmt.Println(err)
            return
        }

        for _, stack := range stacks {
            fmt.Println(stack[0] + " Status: " + stack[1])
        }
    }
}

// snippet-end:[cfn.go.crud_ops.entire]
