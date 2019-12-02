# <a id="Installation"></a>Installation

## Requirements

* Icinga Web 2 (&gt;= 2.8 RC1)
* PHP (&gt;= 5.6, preferably 7.x)
* Icinga Web 2 modules:
  * [reactbundle](https://github.com/Icinga/icingaweb2-module-reactbundle) (>= 0.4) (Icinga Web 2 module)
  * [Icinga PHP Library (ipl)](https://github.com/Icinga/icingaweb2-module-ipl) (???) (Icinga Web 2 module)

## Installation

1. Just drop this module to a `icingadb` subfolder in your Icinga Web 2 module path.

2. Log in with a privileged user in Icinga Web 2 and enable the module in `Configuration -> Modules -> icingadb`.
Or use the `icingacli` and run `icingacli module enable icingadb`.

This concludes the installation. You should now be able to use the new Icinga DB module.
