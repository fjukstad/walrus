# WGS GATK example
Small example pipeline to CountReads and loci in an example WGS dataset. This is
the pipeline outlined in [(howto) RUN the GATK for the first
time](https://software.broadinstitute.org/gatk/documentation/article?id=1209).

It consists of three steps: read input datasets into walrus, run CountReads,
and run CountLoci. walrus will run CountReands and CountLoci in parallel since
they are independent of each other. 

# pipeline.json

```
{ 
    "Name" : "wgs-example" ,
    "Variables": [
        { "Name": "sampleBAM" , "Value":"exampleBAM.bam"},
        { "Name": "sampleFASTA", "Value":"exampleFASTA.fasta"}
    ],
    "Stages": [
            {
            "Name": "input",
            "Image": "ubuntu",
            "Cmd": [
                "sh", "-c",
                "cp /data/{{sample}}* /walrus/input/"
            ],
            "Volumes": ["data:/data"],
            "Cache": true
        },
        {
            "Name": "CountReads",
            "Image": "fjukstad/gatk",
            "Cmd": [
                "-T", "CountReads", "-R", "/walrus/input/{{sampleFASTA}}", "-I",
                "/walrus/input/{{sampleBAM}}" 
            ],
            "Inputs": ["input"]
        },
        {
            "Name": "CountLoci",
            "Image": "fjukstad/gatk",
            "Cmd": [
                "-T", "CountLoci", "-R", "/walrus/input/{{sampleFASTA}}", "-I",
                "/walrus/input/{{sampleBAM}}", "-o","/walrus/CountLoci/output.txt"
            ],
            "Inputs": ["input"]
        }
    ]
}
```


# Run 

```
$ walrus -version-control=false
input completed successfully.
CountReads completed successfully.
CountLoci completed successfully.
All stages completed successfully. 
Output written to ...
```
