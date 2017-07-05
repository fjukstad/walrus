# Fruit stand with variables
Simple pipeline example built after the [fruit stand
example](http://pachyderm.readthedocs.io/en/latest/getting_started/beginner_tutorial.html)
from  great guys at [Pachyderm](http://pachyderm.io/). Look at
[pipeline.json](pipeline.json) for more details on the pipeline. Note that we
use variable names for fruits, docker images and the input path. 

# Run

```
$ walrus -f pipeline.json
input completed successfully.
filter completed successfully.
sum completed successfully.
All stages completed successfully. 
Output written to ...
```

