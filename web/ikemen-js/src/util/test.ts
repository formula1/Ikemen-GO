
import { createSimpleEmitter } from "./SimpleEvent";

const emitter = createSimpleEmitter<[string, number]>()

const off1 = emitter((hello, num)=>{
  console.log("Using it as a function", hello, num)

})

let listener: (a: string, b: number)=>void;
emitter.on(listener = (a, b)=>{
  console.log("Using on", a, b)
})

const off2 = emitter.onReturnOff((a, b)=>{
  console.log("Using onReturnOff", a, b)
})

emitter.emit("Hello", 123)

off1()
emitter.emit("Hello2", 456)

emitter.off(listener)
emitter.emit("Hello3", 789)


off2()
emitter.emit("Shouldn't show", 0)

console.log(emitter.name);
console.log(emitter.constructor);
