import ctypes,json,sys,time
from pathlib import Path
lib=ctypes.CDLL('/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices')
cf=ctypes.CDLL('/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation')
class Point(ctypes.Structure): _fields_=[('x',ctypes.c_double),('y',ctypes.c_double)]
lib.AXIsProcessTrusted.restype=ctypes.c_bool
trusted=lib.AXIsProcessTrusted()
if len(sys.argv)<4: print(json.dumps({'accessibilityTrusted':trusted}));sys.exit(0)
if not trusted: print(json.dumps({'error':'Accessibility unavailable'}));sys.exit(2)
x,y=float(sys.argv[1]),float(sys.argv[2]);action=sys.argv[3]
lib.CGEventCreateMouseEvent.argtypes=[ctypes.c_void_p,ctypes.c_uint32,Point,ctypes.c_uint32];lib.CGEventCreateMouseEvent.restype=ctypes.c_void_p
lib.CGEventPost.argtypes=[ctypes.c_uint32,ctypes.c_void_p];cf.CFRelease.argtypes=[ctypes.c_void_p]
def emit(kind,px,py):
 event=lib.CGEventCreateMouseEvent(None,kind,Point(px,py),0);lib.CGEventPost(0,event);cf.CFRelease(event)
emit(5,x,y);time.sleep(.08);emit(1,x,y);time.sleep(.12)
if action=='drag':
 for i in range(1,9):emit(6,x+4*i,y+3*i);time.sleep(.04)
 emit(2,x+32,y+24)
else:emit(2,x,y)
print(json.dumps({'accessibilityTrusted':True,'method':'Quartz CGEventPost kCGHIDEventTap','action':action,'x':x,'y':y}))
