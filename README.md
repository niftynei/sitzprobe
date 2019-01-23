# Sitzprobe: a Lightning Network payment rehearsal utility

*Sitzprobe*, noun: (from the German for seated rehearsal) is a rehearsal where the singers sing with the orchestra, focusing attention on integrating the two groups. It is often the first rehearsal where the orchestra and singers rehearse together.[source][wiki]


`sitzprobe` is a c-lightning plugin that actively sends test payments through the lightning network. The goal is to improve the health of the network by closing channels that are no able to route payment.


### Installation

Download the source and build it 

```
git clone https://github.com/niftynei/sitzprobe.git
cd sitzprobe
go build
```

This should build a binary named `sitzprobe`. To run lightningd with `sitzprobe` enabled you can either add it to your `lightningd/plugin` directory, or pass it in as a runtime flag.

Option A: symlink to the lightningd plugin directory
```
ln -s sitzprobe /path/to/lightningd/sourcecode/plugin
# then run lightningd normally
./path/to/lightningd/sourcecode/lightningd/lightningd --sitzprobe-freq=20 --sitzprobe-amt=10
``

Option B: tell lightningd what plugins to use at runtime
```
lightningd --plugin=/path/to/sitzprobe --sitzprobe-freq=20 --sitzprobe-amt=10
```

### Options

`sitzprobe` provides two configurable options: 

  - `--sitzprobe-amt`: Amount to send as a test payment, in millisatoshi. Defaults to 1
  - `--sitzprobe-freq`: How frequently to send a test payment, in minutes. Defaults to 60


### Reporting

`sitzprobe` provides a report functionality, to tell you how the payment attempts are working. You can get back a json formatted report by running `sitzprobe-report`.
Note, this feature is currently incomplete.

```
lightning-cli sitzprobe-report
```


### Contributing

Pull requests, issues and feature requests welcome.


[wiki]: https://en.wikipedia.org/wiki/Sitzprobe
