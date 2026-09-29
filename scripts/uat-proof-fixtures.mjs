// Verbatim sanitized rendering event from browser/transcript.json, lines 3928-3971,
// run facet-uat-1788677076660 (2026-09-06). No credentials or session-token URLs.
// Kept in uat*.mjs so the existing Docker test COPY includes this regression data.
export const project = '/home/facet/.facet/productions/uat-agent-1788677488128';
export const completedRender = {
  "type": "cc",
  "data": {
    "raw": {
      "part": {
        "callID": "call_8115d091a16543f9ba5f01f2",
        "id": "prt_0757e9d54001CIvL3XVXADM20I",
        "messageID": "msg_0757e9526001Tb5tdpSwdOCLre",
        "sessionID": "ses_f8a837014ffeqCqz40vPXlqxTB",
        "state": {
          "input": {
            "command": "cd /home/facet/.facet/productions/uat-agent-1788677488128 && facet tools run video_compose --input artifacts/explainer_props.json",
            "timeout": 300000
          },
          "metadata": {
            "exit": 0,
            "output": "{\n  \"ok\": true,\n  \"tool\": \"video_compose\",\n  \"operation\": \"run\",\n  \"result\": {\n    \"composition_id\": \"TitleCard\",\n    \"operation\": \"remotion_render\",\n    \"output\": \"/home/facet/.facet/productions/uat-agent-1788677488128/renders/final.mp4\"\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
            "truncated": false
          },
          "output": "{\n  \"ok\": true,\n  \"tool\": \"video_compose\",\n  \"operation\": \"run\",\n  \"result\": {\n    \"composition_id\": \"TitleCard\",\n    \"operation\": \"remotion_render\",\n    \"output\": \"/home/facet/.facet/productions/uat-agent-1788677488128/renders/final.mp4\"\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
          "status": "completed",
          "time": { "end": 1788677642219, "start": 1788677628142 },
          "title": "cd /home/facet/.facet/productions/uat-agent-1788677488128 && facet tools run video_compose --input artifacts/explainer_props.json"
        },
        "tool": "bash",
        "type": "tool"
      },
      "sessionID": "ses_f8a837014ffeqCqz40vPXlqxTB",
      "timestamp": 1788677642251,
      "type": "tool_use"
    },
    "session_id": "ses_f8a837014ffeqCqz40vPXlqxTB",
    "tool_id": "call_8115d091a16543f9ba5f01f2",
    "tool_input": {
      "command": "cd /home/facet/.facet/productions/uat-agent-1788677488128 && facet tools run video_compose --input artifacts/explainer_props.json",
      "timeout": 300000
    },
    "tool_name": "bash",
    "tool_output": "{\n  \"ok\": true,\n  \"tool\": \"video_compose\",\n  \"operation\": \"run\",\n  \"result\": {\n    \"composition_id\": \"TitleCard\",\n    \"operation\": \"remotion_render\",\n    \"output\": \"/home/facet/.facet/productions/uat-agent-1788677488128/renders/final.mp4\"\n  },\n  \"warnings\": [],\n  \"execution\": {\n    \"provider\": \"local\",\n    \"network\": false,\n    \"external_write\": false,\n    \"estimated_cost\": 0,\n    \"actual_cost\": 0\n  }\n}\n",
    "type": "tool_result"
  }
};
