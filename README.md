# go-autograder
CS 134 Autograder, made by Alec Machlis (@burturt). Portions of this code is based off of https://github.com/nthnluu/go-autograder, with heavy modifications.

## Using the Autograder
This autograder is intended to be used by checking out the autograder in a `go-autograder` submodule folder inside of the git repository containing the configuration folders. A template is available at https://github.com/ucla-progsoftsys/go-autograder-template with the configuration files already set up.

## How the Autograder works

During setup, the specified go version will be installed, in addition to running any shell commands specified in a `custom_setup.sh` script.

This autograder works by running each test by name and folder specified in `autograder.config.json`, and giving credit based on the exit code status of the test case to determine whether it passed. Only tests that you specify in `autograder.config.json` are run and sent to Gradescope. 

When the autograder runs, the student's submission will be copied into `/autograder/source/submission`, all existing `_test.go` files will be deleted (to prevent students from providing their own test cases), and then files inside of `replacement_files` will be overlayed over the student's submission, such as your own `test_test.go` files. You can make any necessary changes to a student's submission not possible with this folder -- such as ensuring parts of a file are unchanged -- before the autograder runs by adding shell commands to `custom_run_autograder.sh`.

## File hierarchy
- `setup.sh` - A setup (Bash) script that installs all your dependencies.
- `run_autograder` - An executable script, in any language (with appropriate #! line), that compiles and runs your autograder suite and produces the output in the correct place.
- `src/test_runner` - A Go module containing the code responsible for running `go test` on a student's submission, parsing the results from stdout, and returning a `results.json` file in Gradescope's specified format.
- `create_zip.sh` - A shell utility to be run within a template repository to generate the zip file to upload to Gradescope

## Config files

### `autograder.config.json`
This JSON file is where you will configure your autograder for your particular assignment. In this file, you must specify the names of the tests you want to use for grading, along with associated point values.

```json=
{
    "visibility": "visible", // Optional visibility setting for autograder results: visible, hidden, after_due_date, after_published
    "tests": [
        {
            "name": "TestAddTwoNumbers",  // The name of the test (must match the test name as defined in test files)
            "number": "1.1", // Optional (will just be numbered in order of array if no number given)
            "points": 5, // The point value of the test case
            "visibility": "visible", // Optional visibility setting for test case: visible, hidden, after_due_date, after_published
            "folder": "main", // Optional directory to run go test in, relative to root folder of submission files
            "timeout": "600s", // Optional test timeout for go test command - fails if it goes beyond this time
            "count": 4 // Optional: specify number of times to run test case - if it fails once, entire test case fails. Note that timeouts (if set) are per run, not across all runs in a single test case
        },
        {
            "name": "TestAddTwoNegativeNumbers",
            "number": "1.2",
            "points": 5,
            "visibility": "visible"
        },
        {
            "name": "TestAddNums",
            "number": "2.1",
            "points": 5,
            "visibility": "visible"
        },
        {
            "name": "TestAddNumsOne",
            "number": "2.2",
            "points": 5,
            "visibility": "visible"
        }
    ]
}
```

### replacement_files/
This folder's contents will be overlayed on top of students' submissions before running tests. For example, if students should have a `main/test_test.go` file, make the file `replacement_files/main/test_test.go`, which will replace (or add) that file in the student's code but keep any other files inside of the `main` folder submitted.

### custom_setup.sh
This shell script is run (using `source custom_setup.sh`) during autograder build time. Specify the `GO_VERSION` variable value to the version of go to install.

### custom_run_autograder.sh
This shell script is run after the student's submission is copied into `/autograder/source/submission` and had their files overlayed, but before running the test cases. This can be used to, for example, check integrity of parts of files in the submission, verify file structure, check for extraneous/missing files, or search for known suspicious strings.