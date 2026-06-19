package com.example.notes;

import static org.hamcrest.Matchers.hasSize;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class NoteControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void createAndListNotes() throws Exception {
        mockMvc.perform(post("/api/notes")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"title\":\"hello\",\"content\":\"world\"}"))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.title").value("hello"));

        mockMvc.perform(get("/api/notes"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$", hasSize(1)))
                .andExpect(jsonPath("$[0].title").value("hello"));
    }

    @Test
    void updateAndDeleteNote() throws Exception {
        String body = mockMvc.perform(post("/api/notes")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"title\":\"draft\",\"content\":\"v1\"}"))
                .andExpect(status().isCreated())
                .andReturn()
                .getResponse()
                .getContentAsString();

        JsonNode idNode = new ObjectMapper().readTree(body).get("id");
        long id = idNode.asLong();

        mockMvc.perform(put("/api/notes/" + id)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"title\":\"final\",\"content\":\"v2\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.title").value("final"));

        mockMvc.perform(delete("/api/notes/" + id))
                .andExpect(status().isNoContent());

        mockMvc.perform(get("/api/notes/" + id))
                .andExpect(status().isNotFound());
    }
}
