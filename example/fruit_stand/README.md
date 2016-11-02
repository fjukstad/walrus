# Fruit stand
Simple pipeline example built after the [fruit stand
example](http://pachyderm.readthedocs.io/en/latest/getting_started/beginner_tutorial.html)
from  great guys at [Pachyderm](http://pachyderm.io/). Look at
[pipeline.json](pipeline.json) for more details on the pipeline. 

# Run

```
$ walrus -f pipeline.json
Stage filter completed successfully
Stage sum completed successfully
```

# Have a look at the output

```
$ ls walrus
filter  sum
$ cat walrus/sum/apple
133
```

