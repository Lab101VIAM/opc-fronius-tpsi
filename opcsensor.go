// Package opcsensor implements a sensor which allows reading opc ua nodes through the readings api and write attributes through do_command

package opcsensor

import (
	"context"
	"errors"
	"io"
	"time"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"

	"github.com/google/uuid"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

var (
	Model = resource.NewModel("viam-soleng", "opc-ua", "opcsensor")
)

func init() {
	resource.RegisterComponent(sensor.API, Model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newOPCSensor,
		},
	)
}

// OPC UA client configuration
type Config struct {
	Endpoint    string   `json:"endpoint"`
	NodeIDs     []string `json:"nodeids"`
	CreateJobID bool     `json:"create_job_id"`
}

// Validate validates the config and returns implicit dependencies.
func (cfg *Config) Validate(path string) ([]string, error) {
	// OPC config validation
	if cfg.Endpoint == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "endpoint")
	}

	// TODO: return implicit dependencies if needed as the first value
	return []string{}, nil
}

// Constructor for a custom sensor that creates and returns an opcsensor.
// TODO: update the customSensor struct and the initialization.
func newOPCSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	// This takes the generic resource.Config passed down from the parent and converts it to the
	// model-specific (aka "native") Config structure defined above, making it easier to directly access attributes.
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	// Create a cancelable context for custom sensor
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &opcSensor{
		name:       rawConf.ResourceName(),
		logger:     logger,
		cfg:        conf,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	// The Reconfigure() method changes the values on the customSensor based on the attributes in the component config
	if err := s.Reconfigure(ctx, deps, rawConf); err != nil {
		logger.Error("Error configuring module with ", rawConf)
		return nil, err
	}

	return s, nil
}

// TODO: update the opcSensor struct with any fields you require.
type opcSensor struct {
	name   resource.Name
	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()

	createJobID bool
	job_id      string

	// OPC client
	opcclient *opcua.Client
}

func (s *opcSensor) Name() resource.Name {
	return s.name
}

// Reconfigures the model. Most models can be reconfigured in place without needing to rebuild. If you need to instead create a new instance of the sensor, throw a NewMustBuildError.
func (s *opcSensor) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		s.logger.Warn("Error reconfiguring module with ", err)
		return err
	}

	s.name = conf.ResourceName()
	s.createJobID = cfg.CreateJobID
	s.job_id = ""

	// Update nodeIDs
	s.cfg.NodeIDs = cfg.NodeIDs

	// OPC client init
	// examples: https://github.com/gopcua/opcua/blob/main/examples/read/read.go
	opcclient, err := opcua.NewClient(s.cfg.Endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err != nil {
		s.logger.Error("OPC client not initiated: ", err)
		return err
	}
	s.opcclient = opcclient
	if err := s.opcclient.Connect(ctx); err != nil {
		s.logger.Error(err)
		return err
	}
	s.logger.Debugf("OPC client successfully connected to: ", s.cfg.Endpoint)
	return nil
}

// Read and return sensor values
func (s *opcSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readResponse, err := s.readOPC(ctx)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{}
	for idx, val := range readResponse.Results {
		result[s.cfg.NodeIDs[idx]] = val.Value.Value()
	}

	// If the client is the Viam data manager
	if extra[data.FromDMString] == true {
		// If not data management collector, return underlying stream contents without filtering.
		// TODO: Make filter field configurable
		if result["ns=1;s=PROCESS_ACTIVE"] == true {
			if s.job_id == "" && s.createJobID {
				// Add Job ID if welding process has started
				s.job_id = uuid.New().String()
			}
			result["job_id"] = s.job_id
			return result, nil
		} else {
			if s.createJobID && s.job_id != "" {
				s.job_id = ""
			}
			return nil, data.ErrNoCaptureToStore
		}
	}
	if s.createJobID {
		result["job_id"] = s.job_id
	}
	return result, nil
}

// DoCommand is used to set opc ua attributes
func (s *opcSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if wvs, ok := cmd["write"]; ok {
		if nodes, ok := wvs.(map[string]any); ok {
			nodesToWrite := []*ua.WriteValue{}
			for k, v := range nodes {
				// Check NodeID
				nodeid, err := ua.ParseNodeID(k)
				if err != nil {
					s.logger.Errorf("invalid node id: %v", err)
					return nil, err
				}
				// Convert value
				v, err := ua.NewVariant(v)
				if err != nil {
					s.logger.Errorf("invalid value: %v", err)
					return nil, err
				}

				nwv := ua.WriteValue{
					NodeID:      nodeid,
					AttributeID: ua.AttributeIDValue,
					Value: &ua.DataValue{
						EncodingMask: ua.DataValueValue,
						Value:        v,
					},
				}
				nodesToWrite = append(nodesToWrite, &nwv)
			}
			writeRequest := &ua.WriteRequest{
				NodesToWrite: nodesToWrite,
			}
			resp, err := s.opcclient.Write(ctx, writeRequest)
			if err != nil {
				return nil, err
			} else {
				result := []any{}
				for _, code := range resp.Results {
					result = append(result, ua.StatusCodes[code])
				}
				return map[string]interface{}{"results": result}, nil
			}
		}
	}
	return nil, nil
}

// Close closes the underlying generic.
func (s *opcSensor) Close(ctx context.Context) error {
	s.cancelFunc()
	s.opcclient.Close(ctx)
	return nil
}

func (s *opcSensor) readOPC(ctx context.Context) (*ua.ReadResponse, error) {
	var readIDs []*ua.ReadValueID

	for _, nodeID := range s.cfg.NodeIDs {
		id, err := ua.ParseNodeID(nodeID)
		if err != nil {
			s.logger.Errorf("invalid node id: %v", err)
			return nil, err
		}
		readIDs = append(readIDs, &ua.ReadValueID{NodeID: id})
	}

	req := &ua.ReadRequest{
		MaxAge:             2000,
		NodesToRead:        readIDs,
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}

	var resp *ua.ReadResponse
	for {
		var err error
		resp, err = s.opcclient.Read(ctx, req)
		if err == nil {
			break
		}

		// Following switch contains known errors that can be retried by the user.
		// Best practice is to do it on read operations.
		switch {
		case err == io.EOF && s.opcclient.State() != opcua.Closed:
			// has to be retried unless user closed the connection
			time.After(1 * time.Second)
			continue

		case errors.Is(err, ua.StatusBadSessionIDInvalid):
			// Session is not activated has to be retried. Session will be recreated internally.
			time.After(1 * time.Second)
			continue

		case errors.Is(err, ua.StatusBadSessionNotActivated):
			// Session is invalid has to be retried. Session will be recreated internally.
			time.After(1 * time.Second)
			continue

		case errors.Is(err, ua.StatusBadSecureChannelIDInvalid):
			// secure channel will be recreated internally.
			time.After(1 * time.Second)
			continue

		default:
			s.logger.Errorf("Read failed: %s", err)
		}
	}

	if resp != nil && resp.Results[0].Status != ua.StatusOK {
		s.logger.Errorf("Status not OK: %v", resp.Results[0].Status)
	}
	return resp, nil
}
