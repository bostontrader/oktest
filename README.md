# oktest
In order to test okconnect, okcatbox, and okprobe we need to setup a fairly elaborate environment.  Said environment and testing scenario is very similar across these tools.  In addition, usage of the three prior enumerated tools is greatly convenienced by using the other tools.  It's too much tedious work to try to maintain three separate, but substantially similar testing facilities, when we can factor all that out into oktest.

oktest exists in order to install these tools and run them through an elaborate scenario.  All of the tools are used in oktest and if oktest passes, then we know that all of the tools are properly tested.

