import React, { useEffect, useRef, useState } from 'react';
import { AiFillPushpin } from 'react-icons/ai';
import {useBufferedState} from "./useBufferedState";
let es: EventSource | null;
export const Receiver: React.FC<any> = ({ senderStreamID }) => {
  const websocket = useRef<WebSocket>();
  const pcSend = useRef<RTCPeerConnection>();
  const [streams, setStreams] = useState<MediaStream[]>([]);

  const [pinStream, setPinStream] = useState<MediaStream | null>(null);

  const [buffer, push] = useBufferedState(10);

  const url = "http://localhost:7002";

  const handleStreamStart = () => {
    if (!es) {
      es = new EventSource(`${url}/sse`);
      push("event source start");
      es.onmessage = (msg) => {
        push(msg.data);
      };
    }
  };

  const handleStreamStop = () => {
    if (es) {
      es.close();
      push("close event source");
      es = null;
    }
  };

  const sendRequest = async (method: string) => {
    await fetch(`${url}/log`, {
      body: method !== "GET" ? Math.random().toString() : null,
      method
    })
  }

  useEffect(() => {
    handleStartPublishing();
  }, []);

  const [connectionState, setConnectionState] = useState<string>();

  const handleStartPublishing = async () => {
    websocket.current = new WebSocket(
      'ws://127.0.0.1:7001/ws?room_id=join-app'
    );
    // websocket.current = new WebSocket("ws://ec2-54-248-35-65.ap-northeast-1.compute.amazonaws.com:7000/ws");
    pcSend.current = new RTCPeerConnection();

    websocket.current.onopen = () => console.log('connection opened');
    websocket.current.onmessage = async (e) => {
      const response = JSON.parse(e.data);
      if (response.type === 'offer') {
        await pcSend.current?.setRemoteDescription(response);
        const answer = await pcSend.current?.createAnswer();

        if (answer) {
          await pcSend.current?.setLocalDescription(answer);
          await websocket.current?.send(
            JSON.stringify({
              type: 'answer',
              data: answer.sdp,
            })
          );
        }
        console.log('set-remote-description');
      } else if (response.type === 'stream-update') {
        const updateStreams = JSON.parse(response.data);
        setStreams(updateStreams)
      }

      //target=1 -> subscriber(taker)
      if (response.candidate && response.target === 1) {
        pcSend.current?.addIceCandidate(response.candidate);
        console.log('add-ice-candidate');
      }
    };

    pcSend.current.onconnectionstatechange = () => {
      console.log('state: ', pcSend.current?.connectionState);
      setConnectionState(pcSend.current?.connectionState);
    };

    //stream sfu bata aayesi call huncha
    // pcSend.current.ontrack = (e) => {
    //   console.log('streams: ', e.streams);
    //   if (recvVideoRef.current) {
    //     recvVideoRef.current.srcObject = e.streams[0];
    //   }
    // };
    pcSend.current.ontrack = (e) => {
      console.log('got-streams: ', e.streams);
      e.streams[0].onremovetrack = () => {
        console.log('onremove track');
        setStreams((s) => s.filter((str) => str.id !== e.streams[0].id));
      };
      setStreams((s) => {
        if (e.streams.length === 1 && e.streams[0].active) {
          if (s.map((s) => s.id).indexOf(e.streams[0].id) === -1) {
            s.push(e.streams[0]);
          }
        }
        return s;
      });
    };

    pcSend.current.onicecandidate = (event) => {
      if (event.candidate) {
        websocket.current?.send(
          JSON.stringify({
            type: 'trickle',
            data: JSON.stringify({
              target: 1,
              candidates: event.candidate,
            }),
          })
        );
      }
    };
  };

  return (
      <div className="grid grid-cols-3 gap-x-10 gap-y-10 p-20">
        <h1>Event Streaming </h1>
        <button onClick={handleStreamStart}>Start Streaming</button>
        <button onClick={handleStreamStop}>Stop Streaming</button>
        <br/>
        <br/>
        <button onClick={() => sendRequest("POST")}>POST TO /log</button>
        <button onClick={() => sendRequest("GET")}>GET TO /log</button>
        <button onClick={() => sendRequest("DELETE")}>
          DELETE TO /log
        </button>
        <button onClick={() => sendRequest("PATCH")}>
          PATCH TO /log
        </button>
        <br/>
        <div className="messages">
          messages
          {buffer.map((d, i) => (
              <pre key={i + Math.random()}>{d}</pre>
          ))}
        </div>
        {connectionState !== 'connected' && (
            <p className="fixed right-1/2 top-1/2 transform translate-x-1/2 -translate-y-1/2 text-white">
              RETREIVING VIDEOS...
            </p>
        )}

        {streams
            .filter((s) => s.id !== senderStreamID && s.active)
            .map((stream, index) => (
                <div
                    className={` rounded-3xl overflow-hidden bg-gray-900 group transition duration-500 ${
                        pinStream === stream
                            ? 'max-h-screen fixed left-0 top-0 h-screen w-screen'
                            : 'relative w-full h-full max-h-96'
                    }`}
                    key={stream.id}
                >
                  <Video srcObject={stream}/>
                  <p className="absolute text-white top-0 left-3">
                    FRIEND {index + 1}
                  </p>
                  <p
                      className="cursor-pointer absolute text-white top-1/2 bg-gray-800 px-2 left-1/2 text-2xl hidden group-hover:block "
                      onClick={() =>
                          pinStream ? setPinStream(null) : setPinStream(stream)
                      }
                  >
                    <AiFillPushpin/>
                  </p>
                </div>
            ))}
      </div>
  );
};

const Video: React.FC<any> = ({srcObject}) => {
  const recvVideoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    if (srcObject && recvVideoRef.current) {
      recvVideoRef.current.srcObject = srcObject;
    }
  }, [srcObject]);

  if (srcObject.active) {
    return (
        <video
            autoPlay
            ref={recvVideoRef}
            className="object-cover w-full h-full"
        ></video>

    )
        ;
  }

  return null;
};
