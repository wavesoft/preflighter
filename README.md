# preflighter
A utility for executing interactive pre-flight checks

![example](docs/preflighter.gif)

## Installation

```
go get github.com/wavesoft/preflighter/preflighter
```

## Usage

Running in interactive mode:

```
preflighter path/to/checklist.yaml
```

Running in unattended (auto) mode (eg. batch):

```
preflighter -a path/to/checklist.yaml
```

