import { createSimpleEmitter } from "../util/SimpleEvent";

const MANDATORY_EXTENSIONS = [
  "OES_texture_float",
  "WEBGL_depth_texture",
  "OES_standard_derivatives",
]

const OPTIONAL_EXTENSIONS = [
  "OES_vertex_array_object"
]


export class Graphics {
  public canvas = document.createElement("canvas");
  public gl: WebGL2RenderingContext


  fullScreenEvent = createSimpleEmitter<[boolean]>();

  documentFullscreenListener: ()=>any

  constructor(){
    const gl = this.canvas.getContext("webgl2");
    if(!gl) throw new Error('Unable to initialize WebGL');
    this.gl = gl;

    for(const ext of MANDATORY_EXTENSIONS){
      if(!gl.getExtension(ext)){
        throw new Error(`Missing mandatory extension: ${ext}`);
      }
    }

    for(const ext of OPTIONAL_EXTENSIONS){
      if(!gl.getExtension(ext)){
        console.warn(`Missing optional extension: ${ext}`);
      }
    }

    this.canvas.tabIndex = -1
    document.addEventListener(
      "fullscreenchange", this.documentFullscreenListener = ()=>{
        this.fullScreenEvent.emit(
          document.fullscreenElement === this.canvas
        )
      }
    );
  }

  destroy(){
    document.removeEventListener(
      "fullscreenchange", this.documentFullscreenListener
    )
    this.canvas.remove();

  }

  getSize(){
    return [this.canvas.width, this.canvas.height]
  }

  toggleFullScreen(){
    if (document.fullscreenElement === this.canvas) {
      document.exitFullscreen();
    } else {
      this.canvas.requestFullscreen();
    }
  }

}
