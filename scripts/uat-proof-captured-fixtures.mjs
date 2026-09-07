// Bounded verbatim sanitized completed events, not a new runtime success.
// Source: facet-uat-1788744802494/evidence/browser/transcript.json (2026-09-07),
// lines 3604-3646, 4137-4179, 4511-4553. The last rewrite is NOT proof.
// Independent delivered hash is from that run's browser/result.json failed_candidate.facts.
export const latestProject = '/home/facet/.facet/productions/uat-agent-1788745177319';
export const deliveredFacts = {"sha256":"4877667a3ba038f21245aef25b19a1e72538933f1fc5ef058a80723384985988"};
export const [failedSourceEdit, estimatedAudioMix, filteredRewrite] = [
  {
    "type": "cc",
    "data": {
      "raw": {
        "part": {
          "callID": "call_61684460cb5a4d6a86fd526c",
          "id": "prt_079890f030016PFfGfLSexgUMO",
          "messageID": "msg_0798909d4001D5tbCEj04Pj5MF",
          "sessionID": "ses_f867a9573ffeXgyLlN9p3fwgJO",
          "state": {
            "input": {
              "command": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools run source_edit --input artifacts/source_edit.json 2>&1"
            },
            "metadata": {
              "exit": 1,
              "output": "{\n  \"ok\": false,\n  \"tool\": \"source_edit\",\n  \"operation\": \"run\",\n  \"error\": {\n    \"code\": \"command_failed\",\n    \"message\": \"ffmpeg failed\",\n    \"retryable\": false,\n    \"details\": {\n      \"error\": \"exit status 1\",\n      \"stderr\": \"Filter concat has an unconnected output\\n\"\n    }\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
              "truncated": false
            },
            "output": "{\n  \"ok\": false,\n  \"tool\": \"source_edit\",\n  \"operation\": \"run\",\n  \"error\": {\n    \"code\": \"command_failed\",\n    \"message\": \"ffmpeg failed\",\n    \"retryable\": false,\n    \"details\": {\n      \"error\": \"exit status 1\",\n      \"stderr\": \"Filter concat has an unconnected output\\n\"\n    }\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
            "status": "completed",
            "time": {
              "end": 1788745421432,
              "start": 1788745420803
            },
            "title": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools run source_edit --input artifacts/source_edit.json 2>&1"
          },
          "tool": "bash",
          "type": "tool"
        },
        "sessionID": "ses_f867a9573ffeXgyLlN9p3fwgJO",
        "timestamp": 1788745421466,
        "type": "tool_use"
      },
      "session_id": "ses_f867a9573ffeXgyLlN9p3fwgJO",
      "tool_id": "call_61684460cb5a4d6a86fd526c",
      "tool_input": {
        "command": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools run source_edit --input artifacts/source_edit.json 2>&1"
      },
      "tool_name": "bash",
      "tool_output": "{\n  \"ok\": false,\n  \"tool\": \"source_edit\",\n  \"operation\": \"run\",\n  \"error\": {\n    \"code\": \"command_failed\",\n    \"message\": \"ffmpeg failed\",\n    \"retryable\": false,\n    \"details\": {\n      \"error\": \"exit status 1\",\n      \"stderr\": \"Filter concat has an unconnected output\\n\"\n    }\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
      "type": "tool_result"
    }
  },
  {
    "type": "cc",
    "data": {
      "raw": {
        "part": {
          "callID": "call_f51ee59423c44452b5334275",
          "id": "prt_079896210001BY8r0jjG1tZhpF",
          "messageID": "msg_079895ebf001r61OvHCEFNf43q",
          "sessionID": "ses_f867a9573ffeXgyLlN9p3fwgJO",
          "state": {
            "input": {
              "command": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools estimate audio_mix --input artifacts/audio_mix.json 2>&1 && facet tools run audio_mix --input artifacts/audio_mix.json 2>&1"
            },
            "metadata": {
              "exit": 0,
              "output": "{\n  \"ok\": true,\n  \"tool\": \"audio_mix\",\n  \"operation\": \"estimate\",\n  \"result\": {\n    \"estimated_cost\": 0,\n    \"external_write\": false,\n    \"network\": false,\n    \"operations\": [\n      \"validate_paths_and_audio_parameters\",\n      \"ffprobe\",\n      \"mix\"\n    ],\n    \"requested_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ],\n    \"side_effect_free\": true,\n    \"validation_scope\": \"request shape, paths, gains, fades, ducking, and loudness ranges; stream presence and fade fit against decoded duration are validated during run\"\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n{\n  \"ok\": true,\n  \"tool\": \"audio_mix\",\n  \"operation\": \"run\",\n  \"result\": {\n    \"duration\": 3,\n    \"loudnorm\": {},\n    \"output\": \"renders/final.mp4\",\n    \"output_facts\": {\n      \"audio\": [\n        {\n          \"bit_rate\": 127035,\n          \"channel_layout\": \"stereo\",\n          \"channels\": 2,\n          \"codec\": \"aac\",\n          \"index\": 1,\n          \"sample_rate\": 48000\n        }\n      ],\n      \"audio_streams\": [\n        {\n          \"bit_rate\": 127035,\n          \"channel_layout\": \"stereo\",\n          \"channels\": 2,\n          \"codec\": \"aac\",\n          \"index\": 1,\n          \"sample_rate\": 48000\n        }\n      ],\n      \"format\": {\n        \"bit_rate\": 146541,\n        \"duration\": 3,\n        \"format_long_name\": \"QuickTime / MOV\",\n        \"format_name\": \"mov,mp4,m4a,3gp,3g2,mj2\",\n        \"size\": 54953\n      },\n      \"input\": \"renders/.videokit-92088775.mp4\",\n      \"sha256\": \"0d68d579a62bf1377c5a07fc94ea27ba1fc38490be25006eae867947736e7569\",\n      \"video\": {\n        \"codec\": \"h264\",\n        \"fps\": 24,\n        \"height\": 180,\n        \"index\": 0,\n        \"pixel_format\": \"yuv420p\",\n        \"reported_frame_rate\": 24,\n        \"rotation\": 0,\n        \"width\": 320\n      },\n      \"video_streams\": [\n        {\n          \"codec\": \"h264\",\n          \"fps\": 24,\n          \"height\": 180,\n          \"index\": 0,\n          \"pixel_format\": \"yuv420p\",\n          \"reported_frame_rate\": 24,\n          \"rotation\": 0,\n          \"width\": 320\n        }\n      ],\n      \"warnings\": []\n    },\n    \"realized_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ],\n    \"requested_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ]\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
              "truncated": false
            },
            "output": "{\n  \"ok\": true,\n  \"tool\": \"audio_mix\",\n  \"operation\": \"estimate\",\n  \"result\": {\n    \"estimated_cost\": 0,\n    \"external_write\": false,\n    \"network\": false,\n    \"operations\": [\n      \"validate_paths_and_audio_parameters\",\n      \"ffprobe\",\n      \"mix\"\n    ],\n    \"requested_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ],\n    \"side_effect_free\": true,\n    \"validation_scope\": \"request shape, paths, gains, fades, ducking, and loudness ranges; stream presence and fade fit against decoded duration are validated during run\"\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n{\n  \"ok\": true,\n  \"tool\": \"audio_mix\",\n  \"operation\": \"run\",\n  \"result\": {\n    \"duration\": 3,\n    \"loudnorm\": {},\n    \"output\": \"renders/final.mp4\",\n    \"output_facts\": {\n      \"audio\": [\n        {\n          \"bit_rate\": 127035,\n          \"channel_layout\": \"stereo\",\n          \"channels\": 2,\n          \"codec\": \"aac\",\n          \"index\": 1,\n          \"sample_rate\": 48000\n        }\n      ],\n      \"audio_streams\": [\n        {\n          \"bit_rate\": 127035,\n          \"channel_layout\": \"stereo\",\n          \"channels\": 2,\n          \"codec\": \"aac\",\n          \"index\": 1,\n          \"sample_rate\": 48000\n        }\n      ],\n      \"format\": {\n        \"bit_rate\": 146541,\n        \"duration\": 3,\n        \"format_long_name\": \"QuickTime / MOV\",\n        \"format_name\": \"mov,mp4,m4a,3gp,3g2,mj2\",\n        \"size\": 54953\n      },\n      \"input\": \"renders/.videokit-92088775.mp4\",\n      \"sha256\": \"0d68d579a62bf1377c5a07fc94ea27ba1fc38490be25006eae867947736e7569\",\n      \"video\": {\n        \"codec\": \"h264\",\n        \"fps\": 24,\n        \"height\": 180,\n        \"index\": 0,\n        \"pixel_format\": \"yuv420p\",\n        \"reported_frame_rate\": 24,\n        \"rotation\": 0,\n        \"width\": 320\n      },\n      \"video_streams\": [\n        {\n          \"codec\": \"h264\",\n          \"fps\": 24,\n          \"height\": 180,\n          \"index\": 0,\n          \"pixel_format\": \"yuv420p\",\n          \"reported_frame_rate\": 24,\n          \"rotation\": 0,\n          \"width\": 320\n        }\n      ],\n      \"warnings\": []\n    },\n    \"realized_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ],\n    \"requested_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ]\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
            "status": "completed",
            "time": {
              "end": 1788745442824,
              "start": 1788745442161
            },
            "title": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools estimate audio_mix --input artifacts/audio_mix.json 2>&1 && facet tools run audio_mix --input artifacts/audio_mix.json 2>&1"
          },
          "tool": "bash",
          "type": "tool"
        },
        "sessionID": "ses_f867a9573ffeXgyLlN9p3fwgJO",
        "timestamp": 1788745442874,
        "type": "tool_use"
      },
      "session_id": "ses_f867a9573ffeXgyLlN9p3fwgJO",
      "tool_id": "call_f51ee59423c44452b5334275",
      "tool_input": {
        "command": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools estimate audio_mix --input artifacts/audio_mix.json 2>&1 && facet tools run audio_mix --input artifacts/audio_mix.json 2>&1"
      },
      "tool_name": "bash",
      "tool_output": "{\n  \"ok\": true,\n  \"tool\": \"audio_mix\",\n  \"operation\": \"estimate\",\n  \"result\": {\n    \"estimated_cost\": 0,\n    \"external_write\": false,\n    \"network\": false,\n    \"operations\": [\n      \"validate_paths_and_audio_parameters\",\n      \"ffprobe\",\n      \"mix\"\n    ],\n    \"requested_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ],\n    \"side_effect_free\": true,\n    \"validation_scope\": \"request shape, paths, gains, fades, ducking, and loudness ranges; stream presence and fade fit against decoded duration are validated during run\"\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n{\n  \"ok\": true,\n  \"tool\": \"audio_mix\",\n  \"operation\": \"run\",\n  \"result\": {\n    \"duration\": 3,\n    \"loudnorm\": {},\n    \"output\": \"renders/final.mp4\",\n    \"output_facts\": {\n      \"audio\": [\n        {\n          \"bit_rate\": 127035,\n          \"channel_layout\": \"stereo\",\n          \"channels\": 2,\n          \"codec\": \"aac\",\n          \"index\": 1,\n          \"sample_rate\": 48000\n        }\n      ],\n      \"audio_streams\": [\n        {\n          \"bit_rate\": 127035,\n          \"channel_layout\": \"stereo\",\n          \"channels\": 2,\n          \"codec\": \"aac\",\n          \"index\": 1,\n          \"sample_rate\": 48000\n        }\n      ],\n      \"format\": {\n        \"bit_rate\": 146541,\n        \"duration\": 3,\n        \"format_long_name\": \"QuickTime / MOV\",\n        \"format_name\": \"mov,mp4,m4a,3gp,3g2,mj2\",\n        \"size\": 54953\n      },\n      \"input\": \"renders/.videokit-92088775.mp4\",\n      \"sha256\": \"0d68d579a62bf1377c5a07fc94ea27ba1fc38490be25006eae867947736e7569\",\n      \"video\": {\n        \"codec\": \"h264\",\n        \"fps\": 24,\n        \"height\": 180,\n        \"index\": 0,\n        \"pixel_format\": \"yuv420p\",\n        \"reported_frame_rate\": 24,\n        \"rotation\": 0,\n        \"width\": 320\n      },\n      \"video_streams\": [\n        {\n          \"codec\": \"h264\",\n          \"fps\": 24,\n          \"height\": 180,\n          \"index\": 0,\n          \"pixel_format\": \"yuv420p\",\n          \"reported_frame_rate\": 24,\n          \"rotation\": 0,\n          \"width\": 320\n        }\n      ],\n      \"warnings\": []\n    },\n    \"realized_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ],\n    \"requested_operations\": [\n      \"music_mix\",\n      \"music_fades\"\n    ]\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
      "type": "tool_result"
    }
  },
  {
    "type": "cc",
    "data": {
      "raw": {
        "part": {
          "callID": "call_3af5ee0446434e2a8192d526",
          "id": "prt_079898d5c00162tcjhvzv89hC2",
          "messageID": "msg_0798983c1001AogEnKVMBk0PuY",
          "sessionID": "ses_f867a9573ffeXgyLlN9p3fwgJO",
          "state": {
            "input": {
              "command": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools run audio_mix --input artifacts/audio_mix.json 2>&1 | grep -E '\"ok\"|\"output\"|\"duration\"|\"codec\"|\"width\"|\"height\"|\"fps\"|\"sample_rate\"' | head -20 && echo \"--- levels ---\" && ffmpeg -i renders/final.mp4 -af \"volumedetect\" -f null /dev/null 2>&1 | grep -E \"mean_volume|max_volume\""
            },
            "metadata": {
              "exit": 0,
              "output": "  \"ok\": true,\n    \"duration\": 3,\n    \"output\": \"renders/final.mp4\",\n          \"codec\": \"aac\",\n          \"sample_rate\": 48000\n          \"codec\": \"aac\",\n          \"sample_rate\": 48000\n        \"duration\": 3,\n        \"codec\": \"h264\",\n        \"fps\": 24,\n        \"height\": 180,\n        \"width\": 320\n          \"codec\": \"h264\",\n          \"fps\": 24,\n          \"height\": 180,\n          \"width\": 320\n--- levels ---\n[Parsed_volumedetect_0 @ 0x595966a75dc0] mean_volume: -18.4 dB\n[Parsed_volumedetect_0 @ 0x595966a75dc0] max_volume: -14.9 dB\n",
              "truncated": false
            },
            "output": "  \"ok\": true,\n    \"duration\": 3,\n    \"output\": \"renders/final.mp4\",\n          \"codec\": \"aac\",\n          \"sample_rate\": 48000\n          \"codec\": \"aac\",\n          \"sample_rate\": 48000\n        \"duration\": 3,\n        \"codec\": \"h264\",\n        \"fps\": 24,\n        \"height\": 180,\n        \"width\": 320\n          \"codec\": \"h264\",\n          \"fps\": 24,\n          \"height\": 180,\n          \"width\": 320\n--- levels ---\n[Parsed_volumedetect_0 @ 0x595966a75dc0] mean_volume: -18.4 dB\n[Parsed_volumedetect_0 @ 0x595966a75dc0] max_volume: -14.9 dB\n",
            "status": "completed",
            "time": {
              "end": 1788745455474,
              "start": 1788745453803
            },
            "title": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools run audio_mix --input artifacts/audio_mix.json 2>&1 | grep -E '\"ok\"|\"output\"|\"duration\"|\"codec\"|\"width\"|\"height\"|\"fps\"|\"sample_rate\"' | head -20 && echo \"--- levels ---\" && ffmpeg -i renders/final.mp4 -af \"volumedetect\" -f null /dev/null 2>&1 | grep -E \"mean_volume|max_volume\""
          },
          "tool": "bash",
          "type": "tool"
        },
        "sessionID": "ses_f867a9573ffeXgyLlN9p3fwgJO",
        "timestamp": 1788745455541,
        "type": "tool_use"
      },
      "session_id": "ses_f867a9573ffeXgyLlN9p3fwgJO",
      "tool_id": "call_3af5ee0446434e2a8192d526",
      "tool_input": {
        "command": "cd /home/facet/.facet/productions/uat-agent-1788745177319 && facet tools run audio_mix --input artifacts/audio_mix.json 2>&1 | grep -E '\"ok\"|\"output\"|\"duration\"|\"codec\"|\"width\"|\"height\"|\"fps\"|\"sample_rate\"' | head -20 && echo \"--- levels ---\" && ffmpeg -i renders/final.mp4 -af \"volumedetect\" -f null /dev/null 2>&1 | grep -E \"mean_volume|max_volume\""
      },
      "tool_name": "bash",
      "tool_output": "  \"ok\": true,\n    \"duration\": 3,\n    \"output\": \"renders/final.mp4\",\n          \"codec\": \"aac\",\n          \"sample_rate\": 48000\n          \"codec\": \"aac\",\n          \"sample_rate\": 48000\n        \"duration\": 3,\n        \"codec\": \"h264\",\n        \"fps\": 24,\n        \"height\": 180,\n        \"width\": 320\n          \"codec\": \"h264\",\n          \"fps\": 24,\n          \"height\": 180,\n          \"width\": 320\n--- levels ---\n[Parsed_volumedetect_0 @ 0x595966a75dc0] mean_volume: -18.4 dB\n[Parsed_volumedetect_0 @ 0x595966a75dc0] max_volume: -14.9 dB\n",
      "type": "tool_result"
    }
  }
];
