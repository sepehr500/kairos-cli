# kairos-cli

**A fast and pleasant cli client for [Temporal](https://github.com/temporalio/temporal)**

<div align="center">
  <video src="https://github.com/user-attachments/assets/9551ec88-6c3a-4c19-9af4-19c989ad8073" width="400" />
</div>

## Why build this?
Although the Temporal team has been doing a great job with the Temporal web UI, it can be sluggish for many use cases. Leaving the terminal can also disrupt flow state for some developers. kairos-cli is a snappy cli application that allows you to maintain your flow state while allowing you to open a workflow in the cloud for a richer experience and deeper analysis. It also adds the following features that Web UI currently lacks

- Updating the status of individual workflows in place.
- Showing if a workflow is stuck by showing the count of underlying retry attempts.
- Updating status counts in real time
- Search with autocomplete

## Features
- Search (with autocomplete)
	- by workflow id
	- by workflow name
	- by workflow status
- Terminate workflows
- Restart workflows
- Show only parent workflow
- Open workflow in temporal cloud
- Dive into a workflow (early testing)

## Installation

```
go install github.com/sepehr500/kairos-cli@latest
```

## Configuration

Create a file called `credentials` in  `~/.config/kairos/` and populate it with your credentials

```
[namespace.default]
    
	temporal_cloud_host="xxx"
	temporal_namespace="xxx"
	temporal_private_key="""
xxxxxxx
"""
    	temporal_public_key="""
xxxxxxxxxxxxxxxxxxx
"""
```

## Run
```
kairos-cli
```
