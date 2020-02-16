package rest

import (
	"github.com/gin-gonic/gin"
	"log"
	"reflect"
	"strconv"
	"strings"
)

type TweakFunc func(r *Rest, c *gin.Context)
type TransactionFunc func(i interface{})

type Model struct {
	name              string
	instance          interface{}
	GetModelFunc      TweakFunc
	GetModelIDFunc    TweakFunc
	PostModelFunc     TweakFunc
	DeleteModelIDFunc TweakFunc
	PutModelIDFunc    TweakFunc
	InstancePool      chan interface{}
	InstanceSlicePool chan interface{}
}
/*
When a request comes, use reflect to generate a instance and do curd with the database.
Using a pool to store several instances generated by reflect before, some requests may be faster.
 */

func NewModel(instance interface{}) *Model {
	t := reflect.TypeOf(instance)
	m :=  &Model{
		name:              strings.ToLower(t.Name()),
		instance:          instance,
	}
	m.SetPoolSize(20)
	m.GetModelFunc = func(r *Rest, c *gin.Context) {
		m.OperateInstanceSlice(func(ms interface{}) {
			if err := r.DB.Find(ms).Error; err == nil {
				c.JSON(200, gin.H{
					"_embedded" : gin.H{
						m.name : ms,
					},
					"_links" : gin.H{
						"self" : gin.H{
							"href" : c.Request.Host + r.BathPath + "/" + m.name,
						},
					},
				})
			}
		})
	}
	m.GetModelIDFunc = func(r *Rest, c *gin.Context) {
		m.OperateInstance(func(mm interface{}) {
			if id, err := strconv.Atoi(c.Param("id")); err == nil {
				if err = r.DB.First(mm, id).Error; err == nil {
					c.JSON(200, mm)
				}
			}
		})
	}
	m.PostModelFunc = func(r *Rest, c *gin.Context) {
		m.OperateInstance(func(mm interface{}) {
			if err := c.BindJSON(mm); err == nil {
				if err = r.DB.Create(mm).Error; err == nil {
					c.JSON(200, mm)
				}
			}
		})
	}
	m.DeleteModelIDFunc = func(r *Rest, c *gin.Context) {
		m.OperateInstance(func(mm interface{}) {
			if id, err := strconv.Atoi(c.Param("id")); err == nil {
				if err = r.DB.First(mm, id).Error; err == nil {
					if err = r.DB.Delete(mm).Error; err == nil {
						c.JSON(200, gin.H{
							"data" : "deleted",
						})
					}
				}
			}
		})
	}
	m.PutModelIDFunc = func(r *Rest, c *gin.Context) {
		m.OperateInstance(func(mm interface{}) {
			if id, err := strconv.Atoi(c.Param("id")); err == nil {
				if err = r.DB.First(mm, id).Error; err == nil {
					if err = c.BindJSON(mm); err == nil {
						if err = r.DB.Save(mm).Error; err == nil {
							c.JSON(200, mm)
						}
					}
				}
			}
		})
	}
	return m
}

func (m *Model) SetPoolSize(size int) {
	instancePool := make(chan interface{}, size)
	instanceSlicePool := make(chan interface{}, size)
	for i := 0; i < size; i++ {
		instancePool<- makeStruct(m.instance)
		instanceSlicePool<- makeSlice(m.instance)
	}
	m.InstancePool = instancePool
	m.InstanceSlicePool = instanceSlicePool
}

func (m *Model) OperateInstance(f TransactionFunc) {
	select {
	case i := <-m.InstancePool:
		f(i)
		m.InstancePool<- i
	default:
		f(makeStruct(m.instance))
		log.Println("make instance by reflect")
	}
}

func (m *Model) OperateInstanceSlice(f TransactionFunc) {
	select {
	case i := <-m.InstanceSlicePool:
		f(i)
		m.InstanceSlicePool<- i
	default:
		f(makeSlice(m.instance))
	}
}

// returns *[]*instance
// Using make() to generate a slice will cause an unaddressed pointer error.
func makeSlice(instance interface{}) interface{} {
	t := reflect.TypeOf(instance)
	slice := reflect.MakeSlice(reflect.SliceOf(t), 10, 10)
	x := reflect.New(slice.Type())
	x.Elem().Set(slice)
	return x.Interface()
}

// returns **instance
func makeStruct(instance interface{}) interface{} {
	st := reflect.TypeOf(instance)
	x := reflect.New(st)
	return x.Interface()
}