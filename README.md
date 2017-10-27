Attempt at a drop-in replacement of the python supervisor program.

There are a number of unsupported / incomplete features. However, for the
purposes that we are using it for it should be complete enough.

It doesn't handle eventlisteners, but it's possible to add this line to

```
[supervisord]
...
exit_on = ANY_FATAL
```

This is non-standard but causes supervisorgo to exit if any of it's processes
fail to start (even after retries) which is what we were using eventlisteners
for before (I think).

Everything gets logged to stdout anyway, so it's no surprise that most of the
logging parameters don't work.

Also, it doesn't daemonize (we don't want it to).

Too many other 'features' that we don't use to mention

It will be necessary to comment out the command in the eventlistener i.e.
```[eventlistener:fatal_check]
#command=bash test_files/bin/exit_on_fatal.sh```

or just remove that section completely :-)

You might want to set [supervisord] logfile to /dev/stdout to see what it's
doing.
