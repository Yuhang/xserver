package  {

    import flash.display.*;
    import flash.events.AsyncErrorEvent;
    import flash.events.IOErrorEvent;
    import flash.events.MouseEvent;
    import flash.events.NetStatusEvent;
    import flash.events.SecurityErrorEvent;
    import flash.net.NetConnection;
    import flash.net.NetStream;
    import flash.text.TextField;

    public class NotifyServerMain extends Sprite {

        import flash.display.Sprite;
        import flash.events.MouseEvent;
        import flash.events.NetStatusEvent;
        import flash.net.NetConnection;
        import flash.net.NetStream;
        import flash.net.URLRequest;
        import flash.net.URLRequestMethod;
        import flash.text.TextField;
        import flash.utils.getTimer;

        public static var statusArea:TextField;

        public static function status(msg:String):void {
            var s:String = statusArea.text;
            var l:int = s.length;
            var m:int = 5000;
            if (l > m) {
                l = l - m;
            } else {
                l = 0;
            }
            statusArea.text = s.substring(l) + msg + "\n";
            statusArea.scrollV = statusArea.maxScrollV;
            trace(msg);
        }

        private var index:int = 0;

        public function NotifyServerMain() {
            stage.frameRate = 60;
            statusArea = new TextField;
            statusArea.x = 0;
            statusArea.y = 40;
            statusArea.border = true;
            statusArea.width = 500;
            statusArea.height = 300;

            this.addChild(statusArea);

            initNativeXNet();
        }

        private var _nc:NetConnection = new NetConnection;
        private var _ns:NetStream = null;

        private function initNativeXNet():void {
            _nc.addEventListener(NetStatusEvent.NET_STATUS, stratusHandler);
            _nc.addEventListener(IOErrorEvent.IO_ERROR, handler);
            _nc.addEventListener(IOErrorEvent.NETWORK_ERROR, handler);
            _nc.addEventListener(AsyncErrorEvent.ASYNC_ERROR, handler);
            _nc.addEventListener(SecurityErrorEvent.SECURITY_ERROR, handler);
            _nc.addEventListener("*", handler);
            _nc.connect("rtmfp://127.0.0.1:1945/introduction");

        }

        private function stratusHandler(e:NetStatusEvent):void {
            status(e.info.code);
            if (e.info.code == "NetConnection.Connect.Success") {
                var xid:Number = e.info.sessionId as Number;
                var client:Object = new Object();
                _nc.client = this;
                _ns = new NetStream(_nc);
                _ns.addEventListener(NetStatusEvent.NET_STATUS, handler);
                _ns.addEventListener(IOErrorEvent.IO_ERROR, handler);
                _ns.addEventListener(AsyncErrorEvent.ASYNC_ERROR, handler);
                _ns.dataReliable = false;
                _ns.client = this;
                _ns.publish("recvPull2");

                var starttime:int = new Date().time;
                statusArea.addEventListener(MouseEvent.CLICK, function(e:MouseEvent):void {
                    status("shoot !!! ");
                    var o:Object = new Object();
                    o.key = "" + index ++;
                    o.time = new Date().time;
                    _ns.send("broadcastBySessionId2", "" + xid, o);
                });

                status("publish!!");
            } else {
                status("test!! " + e.info.code);
            }
        }

        public function recvPull2(... arguments):void {
            status("test2!!");
            var o:Object = arguments[0];
            status("recv2    " + o.key + " " + (new Date().time - o.time));
        }

        public function broadcastBySessionId2(... arguments):void {
            status("test3!!");
            var o:Object = arguments[0];
            status("recv2    " + o.key + " " + (new Date().time - o.time));
        }

        public function recvPull(... arguments):void {
            status("test1!!");
            status("recv1    " + arguments);
        }

        public function handler(e:Object):void {
            status("handler " + e.info.code + " " + e.toString());
        }

    }
}
