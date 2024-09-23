# OPC UA Client modular resource

This module implements an OPC UA client using the Viam sensor api. It allows you to connect to an OPC UA server and read/write its attributes.
The module is work in progress and additional features are planned to be implemented.
As always, please provide feedback and suggestions wherever applicable!

## Requirements

The module doesn't have any specific requirements. If you need support for another platform, let us know.

## Build and run

To use this module, follow the instructions to [add a module from the Viam Registry](https://docs.viam.com/registry/configure/#add-a-modular-resource-from-the-viam-registry) and select the `viam-soleng:opc-ua:opcsensor` model from the [opc-ua-client module](https://app.viam.com/module/viam-soleng/opc-ua-client).

If you plan to modify and build it for yourself, look into check out the Makefile and have a look at [Create Viam Modules](https://docs.viam.com/registry/create/).

## Configure your OPC UA sensor

> [!NOTE]
> Before configuring your sensor you must [create a machine](https://docs.viam.com/manage/fleet/machines/#add-a-new-machine).

Navigate to the **Config** tab of your machine's page in [the Viam app](https://app.viam.com/).
Click on the **Components** subtab and click **Create component**.
Select the `component` type, then select the `opcsensor` model provided by viam-soleng.
Click **Add module**, then enter a name for your sensor and click **Create**.

On the new component panel, copy and paste the following attribute template into your sensor’s **Attributes** box and adjust them according to your environment:

```json
{
  "nodeids": [
    "ns=2;i=3",
    "ns=2;i=2"
  ],
  "endpoint": "opc.tcp://0.0.0.0:4840/freeopcua/server/"
}
```

> [!NOTE]
> For more information, see [Configure a Machine](https://docs.viam.com/manage/configuration/).

### Attributes

The following attributes are available for `<INSERT API NAMESPACE>:<INSERT API NAME>:<INSERT MODEL>` sensor's:

| Name    | Type   | Inclusion    | Description |
| ------- | ------ | ------------ | ----------- |
| `nodeids` | []string | Optional | List of node ids to be read |
| `endpoint` | string | **Required** | The opc ua server address and port |
| `create_job_id` | boolean | Optional | Adds a field `job_id` with a uuid when the welding process is active. Default is false |

### Example configuration

```json
{
  "nodeids": [
    "ns=2;i=3",
    "ns=2;i=2"
  ],
  "endpoint": "opc.tcp://0.0.0.0:4840/freeopcua/server/"
}
```

### Write Attributes

The OPC UA client module allows you to set nodes to certain values by using the following parameters 

```json
{
  "write":{
    "ns=2;i=2":100,
    "ns=2;i=3":0.1
  }
}
```

The result will provide a list of values where 0 means succesful.

### Next steps

Let us know any issues / limitations important to you and your project! Simply create an issue and we will get back to you.
We also welcome pull requests which we will review and consider merging!

## Troubleshooting

Current Known Limitations:
- No authentication
- Only reading of attributes is suppported

## Features Planned

- Write attributes
- Authentication
