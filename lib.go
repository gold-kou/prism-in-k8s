package main

func int32Ptr(i int) *int32 {
	u := int32(i)
	return &u
}
