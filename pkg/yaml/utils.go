package yaml

import (
	"errors"
	"fmt"
	"log"
	"os"
)

func checkFile(p string) error {
	fi, err := os.Stat(p)
	if err != nil {
		return errors.New(fmt.Sprint("File missing:", p, "err=", err))
	}
	if fi.IsDir() {
		return errors.New(fmt.Sprint("Is a dir:", p))
	}
	return nil
}

func (this runnableMap) Validate(c Context) error {
	return this.apply_sequential("VALIDATE", c, func(cc Context, rr Runnable) error {
		return rr.Finish(cc)
	})
}

func (this runnableMap) Prepare(c Context) error {
	return this.apply_sequential("PREPARE", c, func(cc Context, rr Runnable) error {
		return rr.Prepare(cc)
	})
}

func (this runnableMap) Execute(c Context) error {
	return this.apply_sequential("EXECUTE", c, func(cc Context, rr Runnable) error {
		return rr.Execute(cc)
	})
}

func (this runnableMap) Finish(c Context) error {
	return this.apply_sequential("FINISH", c, func(cc Context, rr Runnable) error {
		return rr.Finish(cc)
	})
}

func (this runnableMap) apply_sequential(phase string, c Context, f func(Context, Runnable) error) error {
	for k, runnable := range this {
		log.Println(phase, ":", k)
		err := f(c, runnable)
		if err != nil {
			return err
		}
	}
	return nil
}
