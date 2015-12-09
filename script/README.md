### Overview

The convention will be that any script designed to be run inside a container
will have an `ic-` prefix.  Such a script will be `docker run` by the corresponding script sans prefix.

So for example, given the pair `test` and `ic-test` -- you know that `test`
will run `ic-test` inside a container.

#### Scripts that do not have ic- pairings

- `script/image` - builds the docker image
- `script/common` - shared utility vars/functions
