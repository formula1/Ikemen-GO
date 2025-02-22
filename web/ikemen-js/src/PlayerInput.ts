
import { createSimpleEmitter } from "./util/SimpleEvent";

type KeyListener = (key: string, modifiers: number, value: boolean)=>any;

export class PlayerInput {
  isFocused = false;
  keys: { [key: string]: boolean } = {};

  canvasFocusListener: (e: Event)=>any;
  canvasBlurListener: (e: Event)=>any;
  canvasKeydownListener: (e: KeyboardEvent)=>any;
  canvasKeyupListener: (e: KeyboardEvent)=>any;

  keyEvent = createSimpleEmitter<[string, number, boolean]>()


  constructor(private canvas: HTMLCanvasElement){
    canvas.addEventListener(
      'click', () => canvas.focus()
    );
    canvas.addEventListener(
      'focus', this.canvasFocusListener = () => this.isFocused = true
    );
    canvas.addEventListener(
      'blur', this.canvasBlurListener = () =>{
        this.isFocused = false;
        for(let key in this.keys){
          if(!this.keys[key]) continue;
          this.keys[key] = false;
          this.keyEvent.emit(key, 0, false);
        }
      }
    );
  
    this.canvas.addEventListener('keydown', this.canvasKeydownListener = (e) => {
      if(!this.isFocused) return;
      e.preventDefault();
      this.keys[e.code] = true;
      this.keyEvent.emit(e.code, getModifierKeys(e), true);
   });
    this.canvas.addEventListener('keyup', this.canvasKeyupListener = (e) => {
      if(!this.isFocused) return;
      e.preventDefault();
      if(!this.keys[e.code]) return;
      this.keys[e.code] = false;
      this.keyEvent.emit(e.code, getModifierKeys(e), false);
    });
  }


  destroy(){
    this.canvas.removeEventListener('focus', this.canvasFocusListener);
    this.canvas.removeEventListener('blur', this.canvasBlurListener);
    this.canvas.removeEventListener('keydown', this.canvasKeydownListener);
    this.canvas.removeEventListener('keyup', this.canvasKeyupListener);
  }

  pollKeyboard(key: string): boolean {
    if(!(key in this.keys)) return false;
    return this.keys[key];
  }

  getGamepads(){
    return navigator.getGamepads ? navigator.getGamepads() : [];
  }
  
  getGamepadIds(){
    return this.getGamepads().map(gp => gp ? gp.id : null);
  }

  getGamePadByIndex(index: number){
    const gamepads = this.getGamepads();
    if(index < gamepads.length) return null;
    return gamepads[index]
  }

  
  gamepadExists(gamepadIndex: number){
    const gamepads = this.getGamepads();
    return gamepadIndex < gamepads.length
  }

  getGamepadName(gamepadIndex: number){
    const gamepad = this.getGamePadByIndex(gamepadIndex)
    if(gamepad === null) return null;
    return gamepad.id;
  }

  getGamepadId(gamepadIndex: number){
    const gamepad = this.getGamePadByIndex(gamepadIndex)
    if(gamepad === null) return null;
    return gamepad.id;
  }

  getGamepadAxes(gamepadIndex: number){
    const gamepad = this.getGamePadByIndex(gamepadIndex)
    if(gamepad === null) return null;
    return gamepad.axes;
  }

  getGamepadButtons(gamepadIndex: number){
    const gamepad = this.getGamePadByIndex(gamepadIndex)
    if(gamepad === null) return null;
    return gamepad.buttons.map((button)=>(button.pressed ? 1 : 0));
  }

}

function getModifierKeys(e: KeyboardEvent): number {
  let mod = 0;
  if (e.ctrlKey) mod |= 1;  // Maps to ModCtrl
  if (e.altKey) mod |= 2;   // Maps to ModAlt  
  if (e.shiftKey) mod |= 4; // Maps to ModShift
  return mod;
}
