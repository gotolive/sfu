package logger

import (
	"testing"
)

func Test_defaultLogger(t *testing.T) {
	type fields struct {
		level int
		trace string
	}
	type args struct {
		v  []interface{}
		vf string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Default Logger Debug",
			fields: fields{
				level: LevelAll,
				trace: "test",
			},
			args: args{
				v: []interface{}{
					"abc", 1, true,
				},

				vf: "output %s, %d, %v",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &defaultLogger{
				level: tt.fields.level,
				trace: tt.fields.trace,
			}
			l.Debug(tt.args.v...)
			l.Info(tt.args.v...)
			l.Warn(tt.args.v...)
			l.Error(tt.args.v...)

			l.Debugf(tt.args.vf, tt.args.v...)
			l.Infof(tt.args.vf, tt.args.v...)
			l.Warnf(tt.args.vf, tt.args.v...)
			l.Errorf(tt.args.vf, tt.args.v...)
		})
	}
}

func Test_stubLogger(t *testing.T) {
	type fields struct {
		level int
		trace string
	}
	type args struct {
		v  []interface{}
		vf string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Default Logger Debug",
			fields: fields{
				level: LevelAll,
				trace: "test",
			},
			args: args{
				v: []interface{}{
					"abc", 1, true,
				},

				vf: "output %s, %d, %v",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &stubLogger{}
			l.Debug(tt.args.v...)
			l.Info(tt.args.v...)
			l.Warn(tt.args.v...)
			l.Error(tt.args.v...)

			l.Debugf(tt.args.vf, tt.args.v...)
			l.Infof(tt.args.vf, tt.args.v...)
			l.Warnf(tt.args.vf, tt.args.v...)
			l.Errorf(tt.args.vf, tt.args.v...)
		})
	}
}
